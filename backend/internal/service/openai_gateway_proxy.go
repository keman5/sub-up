package service

import (
	"net/http"
	"strings"
)

func (s *OpenAIGatewayService) openAICodexHTTPProxyURL(account *Account, req *http.Request) string {
	return openAICodexAccountProxyURL(account)
}

func (s *OpenAIGatewayService) openAICodexWSProxyURL(account *Account, wsURL string) string {
	return openAICodexAccountProxyURL(account)
}

func openAICodexAccountProxyURL(account *Account) string {
	if account == nil || account.ProxyID == nil || account.Proxy == nil {
		return ""
	}
	return strings.TrimSpace(account.Proxy.URL())
}
