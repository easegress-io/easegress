/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package certmanager

import (
	"reflect"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	typeCert                    = "CERTIFICATE"
	typeKey                     = "RSA PRIVATE KEY"
	defaultRootCertCountry      = "cn"
	defaultRootCertLocality     = "beijing"
	defaultRootCertOrganization = "megaease"
	defaultRsaBits              = 2046
	defaultSerialNumber         = 202100
)

type (

	// CertManager manages the mesh-wide mTLS cert/keys's refreshing, storing into local Etcd.
	CertManager struct {
		Provider    CertProvider
		service     *service.Service
		appCertTTL  time.Duration
		rootCertTTL time.Duration
	}

	// CertProvider is the interface declaring the methods for the Certificate provider, such as
	//   easemesh-self-sign, Valt, and so on.
	CertProvider interface {
		// SignAppCertAndKey  signs a cert, key pair for one service's instance
		SignAppCertAndKey(serviceName string, IP string, ttl time.Duration) (cert *spec.Certificate, err error)

		// SignRootCertAndKey signs a cert, key pair for root
		SignRootCertAndKey(time.Duration) (cert *spec.Certificate, err error)

		// GetAppCertAndKey gets cert and key for one service's instance
		GetAppCertAndKey(serviceName, IP string) (cert *spec.Certificate, err error)

		// GetRootCertAndKey gets root ca cert and key
		GetRootCertAndKey() (cert *spec.Certificate, err error)

		// ReleaseAppCertAndKey releases one service instance's cert and key
		ReleaseAppCertAndKey(serviceName, IP string) error

		// ReleaseRootCertAndKey releases root CA cert and key
		ReleaseRootCertAndKey() error

		// SetRootCertAndKey sets exists app cert
		SetAppCertAndKey(serviceName, IP string, cert *spec.Certificate) error

		// SetRootCertAndKey sets exists root cert into provider
		SetRootCertAndKey(cert *spec.Certificate) error
	}
)

// NewCertManager creates a initialed certmanager.
func NewCertManager(service *service.Service, certProviderType string, appCertTTL, rootCertTTL time.Duration) *CertManager {
	certManager := &CertManager{
		service:     service,
		appCertTTL:  appCertTTL,
		rootCertTTL: rootCertTTL,
	}

	switch certProviderType {
	case spec.CertProviderSelfSign:
		fallthrough
	default:
		certManager.Provider = NewMeshCertProvider()
	}

	go certManager.init()
	return certManager
}

// sign root/ingress/all services certs
func (cm *CertManager) init() {
	err := cm.SignRootCert()
	if err != nil {
		logger.Errorf("certmanager sign root cert failed: %v", err)
		return
	}

	instanceSpecs := cm.service.ListAllServiceInstanceSpecs()
	err = cm.SignServiceInstances(instanceSpecs)
	if err != nil {
		logger.Errorf("certmanager sign all service failed: %v", err)
		return
	}

}

// CleanAllCerts cleans all exist cert records in Mesh Etcd.
func (cm *CertManager) CleanAllCerts() error {
	rootCert := cm.service.GetRootCert()
	if rootCert != nil {
		cm.service.DelRootCert()
		cm.Provider.ReleaseRootCertAndKey()
	}

	instances := cm.service.ListAllServiceInstanceSpecs()
	for _, v := range instances {
		if v != nil {
			cm.service.DelServiceInstanceCert(v.ServiceName, v.InstanceID)
			cm.Provider.ReleaseAppCertAndKey(v.ServiceName, v.IP)
		}
	}

	ingressInstances := cm.service.ListAllIngressControllerInstanceSpecs()
	for _, v := range ingressInstances {
		if v != nil {
			cm.service.DelIngressControllerInstanceCert(v.InstanceID)
			cm.Provider.ReleaseAppCertAndKey(v.ServiceName, v.IP)
		}
	}
	return nil
}

// needSign will check the cert's TTL, if it's expired, then return true.
// also, if some certs' formats are incorrect, then it will return true for resigning.
func (cm *CertManager) needSign(cert *spec.Certificate) bool {
	if cert == nil {
		return true
	}
	timeNow := time.Now()

	signTime, err := time.Parse(time.RFC3339, cert.SignTime)
	if err != nil {
		logger.Errorf("service: %s has invalid sign time: %s, err: %v, need to resign", cert.ServiceName, cert.SignTime, err)
		return true
	}
	gap := timeNow.Sub(signTime)
	ttl, err := time.ParseDuration(cert.TTL)
	if err != nil {
		logger.Errorf("service: %s has invalid cert ttl: %s, err: %v, need to resign", cert.ServiceName, cert.TTL, err)
		return true
	}

	// expired, need resign
	if gap > ttl {
		logger.Infof("service: %s need to resign cert, gap: %s, need to resign", cert.ServiceName, gap.String())
		return true
	}

	return false
}

// SignRootCert signs the root cert, once the root cert had been resigned
// it will cause the whole system's application certs to be resigned.
func (cm *CertManager) SignRootCert() error {
	var err error
	rootCert := cm.service.GetRootCert()

	if cm.needSign(rootCert) {
		rootCert, err = cm.Provider.SignRootCertAndKey(cm.rootCertTTL)
		if err != nil {
			logger.Errorf("sign root cert failed: %v", err)
			return err
		}
		cm.service.PutRootCert(rootCert)
		cm.ForceSignAllServices()
	} else {
		// set cert from Etcd to provider manually
		if providerCert, err := cm.Provider.GetRootCertAndKey(); err != nil || !reflect.DeepEqual(providerCert, rootCert) {
			cm.Provider.SetRootCertAndKey(rootCert)
			cm.ForceSignAllServices()
		}
	}
	logger.Infof("sign root cert ok")
	return nil
}

// SignIngressController signs ingress controller's cert.
func (cm *CertManager) SignIngressController() error {
	var err error
	instanceSpecs := cm.service.ListAllIngressControllerInstanceSpecs()

	for _, ins := range instanceSpecs {
		if ins.Status != spec.ServiceStatusUp {
			logger.Errorf("ingress controller instance %s is not up, release cert", ins.InstanceID)
			cm.service.DelIngressControllerInstanceCert(ins.InstanceID)
			cm.Provider.ReleaseAppCertAndKey(spec.IngressControllerName, ins.IP)
			continue
		}
		cert := cm.service.GetIngressControllerInstanceCert(ins.InstanceID)
		if cm.needSign(cert) {
			cert, err = cm.Provider.SignAppCertAndKey(spec.IngressControllerName, cert.IP, cm.appCertTTL)
			if err != nil {
				logger.Errorf("sign ingress controller failed: %v", err)
				return err
			}
			cm.service.PutIngressControllerInstanceCert(ins.InstanceID, cert)
		} else {
			// set cert from Etcd to provider manually
			if providerCert, err := cm.Provider.GetAppCertAndKey(spec.IngressControllerName, ins.IP); err != nil || !reflect.DeepEqual(providerCert, cert) {
				cm.Provider.SetAppCertAndKey(spec.IngressControllerName, ins.IP, cert)
			}
		}
	}
	logger.Infof("sign ingress controller ok")

	return nil
}

// ForceSignAllServices resigns all services inside mesh regradless it's expired or not.
func (cm *CertManager) ForceSignAllServices() {
	serviceInstanceSpecs := cm.service.ListAllServiceInstanceSpecs()
	for _, v := range serviceInstanceSpecs {
		if v.Status != spec.ServiceStatusUp {
			continue
		}
		newCert, err := cm.Provider.SignAppCertAndKey(v.ServiceName, v.IP, cm.appCertTTL)
		if err != nil {
			logger.Errorf("service: %s sign cert failed, err: %v", v.ServiceName, err)
			continue
		}

		cm.service.PutServiceInstanceCert(v.ServiceName, v.InstanceID, newCert)
	}
	logger.Infof("try to sign len: %d", len(serviceInstanceSpecs))

	ingressControllerInstances := cm.service.ListAllIngressControllerInstanceSpecs()
	for _, v := range ingressControllerInstances {
		if v.Status != spec.ServiceStatusUp {
			continue
		}
		newCert, err := cm.Provider.SignAppCertAndKey(spec.IngressControllerName, v.IP, cm.appCertTTL)
		if err != nil {
			logger.Errorf("ingress controller  sign cert failed, err: %v", v.ServiceName, err)
			continue
		}

		cm.service.PutIngressControllerInstanceCert(v.InstanceID, newCert)
	}
	logger.Infof("try to sign ingresscontroller len: %d", len(ingressControllerInstances))

}

// SignServiceInstances signs services' instances cert by instanceSpecs parameter.
func (cm *CertManager) SignServiceInstances(instanceSpecs []*spec.ServiceInstanceSpec) error {
	for _, v := range instanceSpecs {
		if v.Status != spec.ServiceStatusUp {
			logger.Infof("service: %s instance %s is not up, not need to sign", v.ServiceName, v.InstanceID)
			cm.service.DelIngressControllerInstanceCert(v.InstanceID)
			cm.Provider.ReleaseAppCertAndKey(v.ServiceName, v.IP)
			continue
		}
		originCert := cm.service.GetServiceInstanceCert(v.ServiceName, v.InstanceID)
		if originCert == nil || cm.needSign(originCert) {
			newCert, err := cm.Provider.SignAppCertAndKey(v.ServiceName, v.IP, cm.appCertTTL)
			if err != nil {
				logger.Errorf("%s sign instance: %s cert failed, err: %v", v.ServiceName, v.InstanceID, err)
				continue
			}

			cm.service.PutServiceInstanceCert(v.ServiceName, v.InstanceID, newCert)
		}

		if providerCert, err := cm.Provider.GetAppCertAndKey(v.ServiceName, v.IP); err != nil || !reflect.DeepEqual(originCert, providerCert) {
			// correct the provider's cert value according to Mesh Etcd's
			cm.Provider.SetAppCertAndKey(v.ServiceName, v.IP, originCert)
		}
	}

	logger.Infof("sign all service instance , len: %d", len(instanceSpecs))

	// sign ingress controller
	if err := cm.SignIngressController(); err != nil {
		logger.Errorf("sign ingress controller failed: %v", err)
		return err
	}
	return nil
}
