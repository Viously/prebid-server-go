package sparteo

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/adapters"
	"github.com/prebid/prebid-server/v3/adapters/adapterstest"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuilder verifies that the Builder function correctly creates a bidder instance.
// It checks for errors, ensures the returned bidder is not nil, and confirms that the endpoint
// in the adapter is set according to the configuration.
func TestBuilder(t *testing.T) {
	cfg := config.Adapter{Endpoint: "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"}
	bidder, err := Builder(openrtb_ext.BidderSparteo, cfg, config.Server{GvlID: 1028})

	require.NoError(t, err, "Builder returned an error")
	assert.NotNil(t, bidder, "Bidder is nil")

	sparteoAdapter, ok := bidder.(*adapter)
	require.True(t, ok, "Expected *adapter, got %T", bidder)

	var endpointBuffer bytes.Buffer
	templateData := struct {
		NetworkId string
		Domain    string
		Bundle    string
	}{
		NetworkId: "networkID",
		Domain:    "dev.sparteo.com",
		Bundle:    "com.sparteo.app",
	}
	err = sparteoAdapter.endpoint.Execute(&endpointBuffer, templateData)
	require.NoError(t, err)
	expectedEndpoint := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com&bundle=com.sparteo.app"
	assert.Equal(t, expectedEndpoint, endpointBuffer.String(), "Endpoint is not correctly set")
}

// TestEndpointTemplateMalformed verifies that the Builder returns an error when the endpoint template is malformed.
func TestEndpointTemplateMalformed(t *testing.T) {
	_, buildErr := Builder(openrtb_ext.BidderSmrtconnect, config.Adapter{
		Endpoint: "{{Malformed}}"}, config.Server{ExternalUrl: "http://bid-test.sparteo.com.com", GvlID: 196})

	assert.Error(t, buildErr)
}

// TestJsonSamples runs JSON sample tests using the shared adapterstest framework.
func TestJsonSamples(t *testing.T) {
	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{
		Endpoint: "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}",
	}, config.Server{GvlID: 1028})
	require.NoError(t, err, "Builder returned an error")

	adapterstest.RunJSONBidderTest(t, "sparteotest", bidder)
}

// TestGetMediaType_InvalidJSON verifies that getMediaType returns an error and an empty result
// when the extension JSON is invalid.
func TestGetMediaType_InvalidJSON(t *testing.T) {
	adapter := &adapter{}
	bid := &openrtb2.Bid{
		Ext: json.RawMessage(`invalid-json`),
	}
	result, err := adapter.getMediaType(bid)
	assert.Error(t, err, "Expected error for invalid JSON")
	assert.Equal(t, openrtb_ext.BidType(""), result, "Expected empty result for invalid JSON")
}

// TestGetMediaType_EmptyType verifies that getMediaType returns an error and an empty result
// when the extension JSON is valid but the "type" field is empty.
func TestGetMediaType_EmptyType(t *testing.T) {
	adapter := &adapter{}
	bid := &openrtb2.Bid{
		Ext: json.RawMessage(`{"prebid":{"type":""}}`),
	}
	result, err := adapter.getMediaType(bid)
	assert.Error(t, err, "Expected error for empty type")
	assert.Equal(t, openrtb_ext.BidType(""), result, "Expected empty result for empty type")
}

// TestGetMediaType_NilExt verifies that getMediaType returns an error and an empty result
// when the bid's extension is nil.
func TestGetMediaType_NilExt(t *testing.T) {
	adapter := &adapter{}
	bid := &openrtb2.Bid{
		Ext: nil,
	}
	result, err := adapter.getMediaType(bid)
	assert.Error(t, err, "Expected error for nil extension")
	assert.Equal(t, openrtb_ext.BidType(""), result, "Expected empty result for nil extension")
}

// TestMakeRequests_ResolvesMacros verifies that the adapter correctly resolves macros in the endpoint URL.
func TestMakeRequests_ResolvesMacros(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{
		Endpoint: endpoint,
	}, config.Server{GvlID: 1028})
	require.NoError(t, err, "Builder returned an error")

	in := &openrtb2.BidRequest{
		ID: "req-qp-1",
		Site: &openrtb2.Site{
			Domain: "dev.sparteo.com",
		},
		Imp: []openrtb2.Imp{
			{
				ID:  "imp-1",
				Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`),
			},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1, "Expected exactly one outgoing request")
	assert.Empty(t, errs, "Unexpected adapter errors")

	expectedURI := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com"
	assert.Equal(t, expectedURI, reqs[0].Uri)
}

// TestMakeRequests_AppBundleMacro verifies that the adapter correctly resolves the
// bundle macro in the endpoint URL when app.bundle is present.
func TestMakeRequests_AppBundleMacro(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-app-bundle-1",
		App: &openrtb2.App{
			Domain: "dev.sparteo.com",
			Bundle: "com.sparteo.app",
			Publisher: &openrtb2.Publisher{
				ID: "sparteo",
			},
		},
		Imp: []openrtb2.Imp{{
			ID:  "imp-1",
			Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`),
		}},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	assert.Empty(t, errs)

	expected := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com&bundle=com.sparteo.app"
	assert.Equal(t, expected, reqs[0].Uri, "endpoint should include bundle when app.bundle is present")
}

// TestMakeRequests_NoBundle_NoQueryParam verifies that the adapter does not append
// the bundle query parameter when app.bundle is empty or missing.
func TestMakeRequests_NoBundle_NoQueryParam(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-app-nobundle-1",
		App: &openrtb2.App{
			Domain:    "dev.sparteo.com",
			Publisher: &openrtb2.Publisher{ID: "sparteo"},
		},
		Imp: []openrtb2.Imp{{
			ID:  "imp-1",
			Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`),
		}},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	assert.Empty(t, errs)

	expected := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com"
	assert.Equal(t, expected, reqs[0].Uri, "endpoint should not contain bundle when empty")
}

// TestMakeRequests_SiteDomain verifies that the adapter uses site.Domain when publisher.Domain is not available.
func TestMakeRequests_SiteDomain(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{
		Endpoint: endpoint,
	}, config.Server{GvlID: 1028})
	require.NoError(t, err, "Builder returned an error")

	in := &openrtb2.BidRequest{
		ID: "req-fallback-1",
		Site: &openrtb2.Site{
			Domain: "dev.sparteo.com",
			// Publisher is nil
		},
		Imp: []openrtb2.Imp{
			{
				ID:  "imp-1",
				Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`),
			},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1, "Expected exactly one outgoing request")
	assert.Empty(t, errs, "Unexpected adapter errors")

	expectedURI := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com"
	assert.Equal(t, expectedURI, reqs[0].Uri)
}

// TestMakeRequests_DomainPrecedence_SiteDomainWins verifies that site.domain win over site.publisher.domain and app.domain
func TestMakeRequests_DomainPrecedence_SiteDomainWins(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-domain-1",
		Site: &openrtb2.Site{
			Domain: "site.sparteo.com",
			Publisher: &openrtb2.Publisher{
				Domain: "site-pub.sparteo.com",
			},
		},
		App: &openrtb2.App{
			Domain: "app.sparteo.com",
			Publisher: &openrtb2.Publisher{
				Domain: "app-pub.should-not-be-used",
			},
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	assert.Equal(t,
		"https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=site.sparteo.com",
		reqs[0].Uri,
	)
}

// TestMakeRequests_DomainPrecedence_SitePublisherWhenSiteDomainEmpty checks the fall back to site.publisher.domain when site.domain is empty
func TestMakeRequests_DomainPrecedence_SitePublisherWhenSiteDomainEmpty(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-domain-2",
		Site: &openrtb2.Site{
			Domain: "",
			Publisher: &openrtb2.Publisher{
				Domain: "site-pub.sparteo.com",
			},
		},
		App: &openrtb2.App{
			Domain: "app.sparteo.com",
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	assert.Equal(t,
		"https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=site-pub.sparteo.com",
		reqs[0].Uri,
	)
}

// TestMakeRequests_DomainPrecedence_AppDomainWhenNoSite verifies that if there is no site, it uses app.domain
func TestMakeRequests_DomainPrecedence_AppDomainWhenNoSite(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID:   "req-domain-3",
		Site: nil,
		App: &openrtb2.App{
			Domain: "app.sparteo.com",
			Publisher: &openrtb2.Publisher{
				Domain: "app-pub.should-not-be-used",
			},
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	assert.Equal(t,
		"https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=app.sparteo.com",
		reqs[0].Uri,
	)
}

// --- Helper for reading publisher.ext.params.networkId in output JSON ---
func readNetworkIDFromPublisherExt(t *testing.T, ext json.RawMessage) (string, bool) {
	if len(ext) == 0 {
		return "", false
	}
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(ext, &m))
	params, _ := m["params"].(map[string]interface{})
	if params == nil {
		return "", false
	}
	val, _ := params["networkId"].(string)
	if val == "" {
		return "", false
	}
	return val, true
}

// When site exists but site.publisher is nil, the adapter must create publisher and upsert networkId into site.publisher.ext
func TestMakeRequests_UpdatePublisherExtension_CreatesSitePublisherIfMissing(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-upsert-site",
		Site: &openrtb2.Site{
			Domain:    "site.sparteo.com",
			Publisher: nil, // deliberately nil
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	var out openrtb2.BidRequest
	require.NoError(t, json.Unmarshal(reqs[0].Body, &out))

	require.NotNil(t, out.Site)
	require.NotNil(t, out.Site.Publisher, "publisher should be created when missing")
	val, ok := readNetworkIDFromPublisherExt(t, out.Site.Publisher.Ext)
	require.True(t, ok, "site.publisher.ext.params.networkId should exist")
	assert.Equal(t, "networkID", val)
}

// When no site is present, create app.publisher if missing and upsert networkId into app.publisher.ext
func TestMakeRequests_UpdatePublisherExtension_CreatesAppPublisherIfMissingWhenNoSite(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID:   "req-upsert-app",
		Site: nil,
		App: &openrtb2.App{
			Domain:    "app.sparteo.com",
			Publisher: nil, // deliberately nil
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	var out openrtb2.BidRequest
	require.NoError(t, json.Unmarshal(reqs[0].Body, &out))

	require.Nil(t, out.Site)
	require.NotNil(t, out.App)
	require.NotNil(t, out.App.Publisher, "app.publisher should be created when missing")
	val, ok := readNetworkIDFromPublisherExt(t, out.App.Publisher.Ext)
	require.True(t, ok, "app.publisher.ext.params.networkId should exist")
	assert.Equal(t, "networkID", val)
}

// TestMakeRequests_UpdatePublisherExtension_PrefersSiteOverApp verifies that we prefer Site when both Site and App exist; only Site should receive the networkId
func TestMakeRequests_UpdatePublisherExtension_PrefersSiteOverApp(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.Domain}}{{if .Bundle}}&bundle={{.Bundle}}{{end}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{Endpoint: endpoint}, config.Server{GvlID: 1028})
	require.NoError(t, err)

	in := &openrtb2.BidRequest{
		ID: "req-upsert-prefer-site",
		Site: &openrtb2.Site{
			Domain:    "site.sparteo.com",
			Publisher: &openrtb2.Publisher{}, // exists but empty
		},
		App: &openrtb2.App{
			Domain:    "app.sparteo.com",
			Publisher: &openrtb2.Publisher{}, // also exists
		},
		Imp: []openrtb2.Imp{
			{ID: "imp-1", Ext: json.RawMessage(`{"bidder":{"networkId":"networkID"}}`)},
		},
	}

	reqs, errs := bidder.MakeRequests(in, &adapters.ExtraRequestInfo{})
	require.Len(t, reqs, 1)
	require.Empty(t, errs)

	var out openrtb2.BidRequest
	require.NoError(t, json.Unmarshal(reqs[0].Body, &out))

	// Site should have the networkId
	require.NotNil(t, out.Site)
	require.NotNil(t, out.Site.Publisher)
	siteVal, siteOk := readNetworkIDFromPublisherExt(t, out.Site.Publisher.Ext)
	require.True(t, siteOk)
	assert.Equal(t, "networkID", siteVal)

	// App should NOT be written
	if out.App != nil && out.App.Publisher != nil {
		_, appOk := readNetworkIDFromPublisherExt(t, out.App.Publisher.Ext)
		assert.False(t, appOk, "app.publisher.ext should not contain networkId when site exists")
	}
}
