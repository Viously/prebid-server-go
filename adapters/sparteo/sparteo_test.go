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
	cfg := config.Adapter{Endpoint: "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.PublisherDomain}}"}
	bidder, err := Builder(openrtb_ext.BidderSparteo, cfg, config.Server{GvlID: 1028})

	require.NoError(t, err, "Builder returned an error")
	assert.NotNil(t, bidder, "Bidder is nil")

	sparteoAdapter, ok := bidder.(*adapter)
	require.True(t, ok, "Expected *adapter, got %T", bidder)

	var endpointBuffer bytes.Buffer
	templateData := struct {
		NetworkId       string
		PublisherDomain string
	}{
		NetworkId:       "networkID",
		PublisherDomain: "dev.sparteo.com",
	}
	err = sparteoAdapter.endpoint.Execute(&endpointBuffer, templateData)
	require.NoError(t, err)
	expectedEndpoint := "https://bid-test.sparteo.com/s2s-auction?network_id=networkID&domain=dev.sparteo.com"
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
		Endpoint: "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.PublisherDomain}}",
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
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.PublisherDomain}}"

	bidder, err := Builder(openrtb_ext.BidderSparteo, config.Adapter{
		Endpoint: endpoint,
	}, config.Server{GvlID: 1028})
	require.NoError(t, err, "Builder returned an error")

	in := &openrtb2.BidRequest{
		ID: "req-qp-1",
		Site: &openrtb2.Site{
			Publisher: &openrtb2.Publisher{
				Domain: "dev.sparteo.com",
			},
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

// TestMakeRequests_FallbackToSiteDomain verifies that the adapter uses site.Domain when publisher.Domain is not available.
func TestMakeRequests_FallbackToSiteDomain(t *testing.T) {
	endpoint := "https://bid-test.sparteo.com/s2s-auction?network_id={{.NetworkId}}&domain={{.PublisherDomain}}"

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
