package proxymode

import (
	"net/http"
	"net/url"

	"github.com/elastic/package-registry/packages"
)

type proxyResolver struct {
	artifactsPackagesURL url.URL
	artifactsStaticURL   url.URL
}

func (pr proxyResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	panic("implement me")
}

func (pr proxyResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	panic("implement me")
}

func (pr proxyResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	panic("implement me")
}

var _ packages.RemoteResolver = new(proxyResolver)
