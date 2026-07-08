package downloader

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
)

func createOrderedClientsFromStrategy(opt Options, strategy DownloadStrategy) []*http.Client {
	if opt.Client != nil {
		return []*http.Client{opt.Client}
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: opt.InsecureSkipVerify}

	makeClient := func(proxyURL string) *http.Client {
		tr := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSClientConfig:       tlsConfig,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       30 * time.Second,
			DisableKeepAlives:     true,
		}

		if proxyURL != "" {
			if p, err := url.Parse(proxyURL); err == nil {
				tr.Proxy = http.ProxyURL(p)
			}
		}

		return &http.Client{
			Timeout:   10 * time.Minute,
			Transport: tr,
		}
	}

	var clients []*http.Client

	if strategy.PreferProxy && strategy.ProxyURL != "" {

		clients = append(clients, makeClient(strategy.ProxyURL))
		clients = append(clients, makeClient(""))
		return clients
	}

	clients = append(clients, makeClient(""))
	if strategy.ProxyURL != "" {
		clients = append(clients, makeClient(strategy.ProxyURL))
	}

	return clients
}

func BuildOrderedClients(strategy func() DownloadStrategy, timeout time.Duration) []*http.Client {
	directClient := &http.Client{Timeout: timeout}

	if strategy == nil {
		return []*http.Client{directClient}
	}

	strat := strategy()
	if strat.PreferProxy && strat.ProxyURL != "" {
		return []*http.Client{NewProxyClient(strat.ProxyURL), directClient}
	}

	clients := []*http.Client{directClient}
	if strat.ProxyURL != "" {
		clients = append(clients, NewProxyClient(strat.ProxyURL))
	}
	return clients
}

func NewProxyClient(proxyURL string) *http.Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		DisableKeepAlives:     true,
	}

	if proxyURL != "" {
		if p, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(p)
		}
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: tr,
	}
}
