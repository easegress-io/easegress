package master

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	v1alpha1 "github.com/megaease/easemesh-api/v1alpha1"
)

type ingressesByOrder []*spec.Ingress

func (s ingressesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s ingressesByOrder) Len() int           { return len(s) }
func (s ingressesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readIngressName(ctx iris.Context) (string, error) {
	serviceName := ctx.Params().Get("ingressName")
	if serviceName == "" {
		return "", fmt.Errorf("empty ingress name")
	}

	return serviceName, nil
}

func (m *Master) listIngresses(ctx iris.Context) {
	specs := m.service.ListIngressSpecs()

	sort.Sort(ingressesByOrder(specs))
	var apiSpecs []*v1alpha1.Ingress
	for _, v := range specs {
		ingress := &v1alpha1.Ingress{}
		err := m.convertSpecToPB(v, ingress)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, ingress)
	}
	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "application/json")
	ctx.Write(buff)
}

func (m *Master) createIngress(ctx iris.Context) {
	pbIngressSpec := &v1alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(ctx, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if ingressName != ingressSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", ingressName, ingressSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec != nil {
		api.HandleAPIError(ctx, http.StatusConflict, fmt.Errorf("%s existed", ingressName))
		return
	}

	m.service.PutIngressSpec(ingressSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) getIngress(ctx iris.Context) {
	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	ingressSpec := m.service.GetIngressSpec(ingressName)
	if ingressSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}
	pbIngressSpec := &v1alpha1.Ingress{}
	err = m.convertSpecToPB(ingressSpec, pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", ingressSpec, err))
	}

	buff, err := json.Marshal(pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbIngressSpec, err))
	}

	ctx.Header("Content-Type", "application/json")
	ctx.Write(buff)
}

func (m *Master) updateIngress(ctx iris.Context) {
	pbIngressSpec := &v1alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(ctx, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if ingressName != ingressSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", ingressName, ingressSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	m.service.PutIngressSpec(ingressSpec)
}

func (m *Master) deleteIngress(ctx iris.Context) {
	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	m.service.DeleteIngressSpec(ingressName)
}
