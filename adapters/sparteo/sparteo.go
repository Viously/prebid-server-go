package sparteo

import (
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/adapters"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/errortypes"
	"github.com/prebid/prebid-server/v3/macros"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/prebid/prebid-server/v3/util/jsonutil"
)

type adapter struct {
	endpoint *template.Template
}

type extBidWrapper struct {
	Prebid openrtb_ext.ExtBidPrebid `json:"prebid"`
}

func Builder(bidderName openrtb_ext.BidderName, cfg config.Adapter, server config.Server) (adapters.Bidder, error) {
	template, err := template.New("endpointTemplate").Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse endpoint url template: %v", err)
	}

	bidder := &adapter{
		endpoint: template,
	}
	return bidder, nil
}

func parseExt(imp *openrtb2.Imp) (*openrtb_ext.ExtImpSparteo, error) {
	var bidderExt adapters.ExtImpBidder

	bidderExtErr := jsonutil.Unmarshal(imp.Ext, &bidderExt)
	if bidderExtErr != nil {
		return nil, fmt.Errorf("ignoring imp id=%s, error while decoding extImpBidder, err: %s", imp.ID, bidderExtErr)
	}

	impExt := openrtb_ext.ExtImpSparteo{}
	sparteoExtErr := jsonutil.Unmarshal(bidderExt.Bidder, &impExt)
	if sparteoExtErr != nil {
		return nil, fmt.Errorf("ignoring imp id=%s, error while decoding impExt, err: %s", imp.ID, sparteoExtErr)
	}

	return &impExt, nil
}

func (a *adapter) MakeRequests(req *openrtb2.BidRequest, reqInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	request := *req
	var errs []error

	request.Imp = make([]openrtb2.Imp, len(req.Imp))
	copy(request.Imp, req.Imp)

	if req.Site != nil {
		siteCopy := *req.Site
		request.Site = &siteCopy
		if req.Site.Publisher != nil {
			pubCopy := *req.Site.Publisher
			request.Site.Publisher = &pubCopy
		}
	}
	if req.App != nil {
		appCopy := *req.App
		request.App = &appCopy
		if req.App.Publisher != nil {
			pubCopy := *req.App.Publisher
			request.App.Publisher = &pubCopy
		}
	}

	var domain string
	if request.Site != nil {
		if request.Site.Domain != "" {
			domain = request.Site.Domain
		} else if request.Site.Publisher != nil {
			domain = request.Site.Publisher.Domain
		}
	}
	if domain == "" && request.App != nil {
		domain = request.App.Domain
	}

	var bundle string
	if request.App != nil {
		bundle = request.App.Bundle
	}

	var networkID string
	for i, imp := range request.Imp {
		extImpSparteo, err := parseExt(&imp)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if networkID == "" && extImpSparteo.NetworkId != "" {
			networkID = extImpSparteo.NetworkId
		}

		var extMap map[string]interface{}
		if err := jsonutil.Unmarshal(imp.Ext, &extMap); err != nil {
			errs = append(errs, fmt.Errorf("ignoring imp id=%s, error while unmarshaling ext, err: %s", imp.ID, err))
			continue
		}

		sparteoMap, ok := extMap["sparteo"].(map[string]interface{})
		if !ok {
			sparteoMap = make(map[string]interface{})
			extMap["sparteo"] = sparteoMap
		}

		paramsMap, ok := sparteoMap["params"].(map[string]interface{})
		if !ok {
			paramsMap = make(map[string]interface{})
			sparteoMap["params"] = paramsMap
		}

		if bidderObj, ok := extMap["bidder"].(map[string]interface{}); ok {
			delete(extMap, "bidder")
			for k, v := range bidderObj {
				paramsMap[k] = v
			}
		}

		updatedExt, err := jsonutil.Marshal(extMap)
		if err != nil {
			errs = append(errs, fmt.Errorf("ignoring imp id=%s, error while marshaling updated ext, err: %s", imp.ID, err))
			continue
		}
		request.Imp[i].Ext = updatedExt
	}

	var pub *openrtb2.Publisher
	var pubExt string

	if request.Site != nil {
		pub = ensurePublisher(&request.Site.Publisher)
		pubExt = "site.publisher.ext"
	} else if request.App != nil {
		pub = ensurePublisher(&request.App.Publisher)
		pubExt = "app.publisher.ext"
	}

	ext, err := updatePublisherExtension(&pub.Ext, networkID, pubExt)
	if err != nil {
		errs = append(errs, err)
	} else {
		pub.Ext = ext
	}

	body, err := jsonutil.Marshal(request)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	uri, err := a.buildEndpointURL(networkID, domain, bundle)
	if err != nil {
		return nil, []error{err}
	}

	requestData := &adapters.RequestData{
		Method: http.MethodPost,
		Uri:    uri,
		Body:   body,
		ImpIDs: openrtb_ext.GetImpIDs(request.Imp),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	return []*adapters.RequestData{requestData}, errs
}

func ensurePublisher(p **openrtb2.Publisher) *openrtb2.Publisher {
	log.Println("ensurePublisher", p)
	if *p == nil {
		*p = &openrtb2.Publisher{}
	}

	return *p
}

func updatePublisherExtension(targetExt *jsonutil.RawMessage, networkID, fieldPath string) ([]byte, error) {
	var pubExt map[string]interface{}
	if *targetExt != nil {
		if err := jsonutil.Unmarshal(*targetExt, &pubExt); err != nil {
			pubExt = make(map[string]interface{})
		}
	} else {
		pubExt = make(map[string]interface{})
	}

	log.Println("updatePublisherExtension", pubExt)

	params, ok := pubExt["params"].(map[string]interface{})
	if !ok {
		params = make(map[string]interface{})
		pubExt["params"] = params
	}
	params["networkId"] = networkID

	updated, err := jsonutil.Marshal(pubExt)
	if err != nil {
		return nil, &errortypes.BadInput{
			Message: fmt.Sprintf("Error marshaling %s: %s", fieldPath, err),
		}
	}
	return updated, nil
}

func (a *adapter) buildEndpointURL(networkId string, domain string, bundle string) (string, error) {
	endpointParams := macros.EndpointTemplateParams{NetworkId: networkId, Domain: domain, Bundle: bundle}
	return macros.ResolveMacros(a.endpoint, endpointParams)
}

func (a *adapter) MakeBids(req *openrtb2.BidRequest, reqData *adapters.RequestData, respData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if adapters.IsResponseStatusCodeNoContent(respData) {
		return nil, nil
	}

	if err := adapters.CheckResponseStatusCodeForErrors(respData); err != nil {
		return nil, []error{err}
	}

	var bidResp openrtb2.BidResponse
	if err := jsonutil.Unmarshal(respData.Body, &bidResp); err != nil {
		return nil, []error{err}
	}

	bidderResponse := adapters.NewBidderResponse()
	bidderResponse.Currency = bidResp.Cur

	for _, seatBid := range bidResp.SeatBid {
		for i, bid := range seatBid.Bid {
			bidType, err := a.getMediaType(&bid)
			if err != nil {
				continue
			}

			switch bidType {
			case openrtb_ext.BidTypeBanner:
				seatBid.Bid[i].MType = openrtb2.MarkupBanner
			case openrtb_ext.BidTypeVideo:
				seatBid.Bid[i].MType = openrtb2.MarkupVideo
			case openrtb_ext.BidTypeNative:
				seatBid.Bid[i].MType = openrtb2.MarkupNative
			default:
				continue
			}

			bidderResponse.Bids = append(bidderResponse.Bids, &adapters.TypedBid{
				Bid:     &seatBid.Bid[i],
				BidType: bidType,
			})
		}
	}

	return bidderResponse, nil
}

func (a *adapter) getMediaType(bid *openrtb2.Bid) (openrtb_ext.BidType, error) {
	var wrapper extBidWrapper
	if err := jsonutil.Unmarshal(bid.Ext, &wrapper); err != nil {
		return "", fmt.Errorf("error unmarshaling bid ext for bid id=%s: %v", bid.ID, err)
	}
	bidExt := wrapper.Prebid

	bidType, err := openrtb_ext.ParseBidType(string(bidExt.Type))
	if err != nil {
		return "", fmt.Errorf("error parsing bid type for bid id=%s: %v", bid.ID, err)
	}

	if bidType == openrtb_ext.BidTypeAudio {
		return "", fmt.Errorf("bid type %q is not supported for bid id=%s", bidExt.Type, bid.ID)
	}

	return bidType, nil
}
