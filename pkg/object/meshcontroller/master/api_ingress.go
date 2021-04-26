package master

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"

	"gopkg.in/yaml.v2"
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

	buff, err := yaml.Marshal(specs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) createIngress(ctx iris.Context) {
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, ingressSpec)
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

	buff, err := yaml.Marshal(ingressSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", ingressSpec, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) updateIngress(ctx iris.Context) {
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, ingressSpec)
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
