package pragmatic

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/imroc/req/v3"
)

type PragmaticPlay struct {
	ctx    context.Context
	client *req.Client
	url    *string
}

type SessionData struct {
	RedirectURL string
	MGCKey      string
}

type ResponseData struct {
	Index      int     `json:"index"`
	Counter    int     `json:"counter"`
	Balance    float64 `json:"balance"`
	NextAction string  `json:"na"`
	TotalWin   float64 `json:"total_win"`
}

func NewPragmaticPlay(ctx context.Context, url, ua string) *PragmaticPlay {
	client := req.C().SetTimeout(5 * time.Second)

	client.SetUserAgent(ua)
	client.Headers.Add("accept-encoding", "gzip, deflate, br")
	client.Headers.Add("cache-control", "no-cache")

	return &PragmaticPlay{
		ctx:    ctx,
		client: client,
		url:    &url,
	}
}

func (pp *PragmaticPlay) LoadSession() (*SessionData, error) {
	// Set redirect policy on the client before making the request
	pp.client.SetRedirectPolicy(req.NoRedirectPolicy())

	resp, err := pp.client.R().
		SetContext(pp.ctx).
		Get(*pp.url)

	if err != nil {
		return nil, err
	}

	location := resp.Header.Get("location")
	if location == "" {
		return nil, nil
	}

	u, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	pp.client.SetBaseURL(fmt.Sprintf("https://%s", u.Host))

	return &SessionData{
		RedirectURL: location,
		MGCKey:      u.Query().Get("mgckey"),
	}, nil
}

func (pp *PragmaticPlay) DoInit(mgckey string, symbol string) (*ResponseData, error) {
	resp, err := pp.client.R().
		SetContext(pp.ctx).
		SetHeaders(map[string]string{
			"accept":       "*/*",
			"content-type": "application/x-www-form-urlencoded",
		}).
		SetFormData(map[string]string{
			"action":  "doInit",
			"symbol":  symbol,
			"cver":    "339188",
			"index":   "1",
			"counter": "1",
			"repeat":  "0",
			"mgckey":  mgckey,
		}).
		Post("/gs2c/ge/v4/gameService")

	if err != nil {
		return nil, err
	}

	return pp.ParseResponseData(resp)
}

func (pp *PragmaticPlay) ParseResponseData(resp *req.Response) (*ResponseData, error) {
	body := resp.String()

	getInt := func(key string) int {
		re := regexp.MustCompile(key + `=([0-9]+)`)
		m := re.FindStringSubmatch(body)
		if len(m) > 1 {
			val, _ := strconv.Atoi(m[1])
			return val
		}
		return 0
	}

	getFloat := func(key string) float64 {
		re := regexp.MustCompile(key + `=([0-9.,]+)`)
		m := re.FindStringSubmatch(body)
		if len(m) > 1 {
			// Remove commas before parsing
			clean := regexp.MustCompile(`,`).ReplaceAllString(m[1], "")
			val, _ := strconv.ParseFloat(clean, 64)
			return val
		}
		return 0
	}

	getString := func(key string) string {
		re := regexp.MustCompile(key + `=([^&]*)`)
		m := re.FindStringSubmatch(body)
		if len(m) > 1 {
			return m[1]
		}
		return ""
	}

	return &ResponseData{
		Index:      getInt("index"),
		Counter:    getInt("counter"),
		Balance:    getFloat("balance"),
		TotalWin:   getFloat("tw"),
		NextAction: getString("na"),
	}, nil
}

func (pp *PragmaticPlay) DoSpin(mgckey, symbol string, c, index, counter int, sInfo string) (*ResponseData, error) {
	resp, err := pp.client.R().
		SetContext(pp.ctx).
		SetHeaders(map[string]string{
			"content-type": "application/x-www-form-urlencoded",
		}).
		SetFormData(map[string]string{
			"action":  "doSpin",
			"symbol":  symbol,
			"c":       strconv.Itoa(c),
			"l":       "1024",
			"sInfo":   sInfo,
			"index":   strconv.Itoa(index),
			"counter": strconv.Itoa(counter),
			"repeat":  "0",
			"mgckey":  mgckey,
		}).
		Post("/gs2c/ge/v4/gameService")

	if err != nil {
		return nil, err
	}

	return pp.ParseResponseData(resp)
}

func (pp *PragmaticPlay) DoCollect(mgckey, symbol string, index, counter int) (*ResponseData, error) {
	resp, err := pp.client.R().
		SetContext(pp.ctx).
		SetHeaders(map[string]string{
			"content-type": "application/x-www-form-urlencoded",
		}).
		SetFormData(map[string]string{
			"symbol":  symbol,
			"action":  "doCollect",
			"index":   strconv.Itoa(index),
			"counter": strconv.Itoa(counter),
			"repeat":  "0",
			"mgckey":  mgckey,
		}).
		Post("/gs2c/ge/v4/gameService")

	if err != nil {
		return nil, err
	}

	return pp.ParseResponseData(resp)
}
