package proxy

import (
	"crypto/subtle"
	"net/http"
)

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

func (h *Handler) validateAdminCSRF(r *http.Request, sess *adminSession) bool {
	if sess == nil || sess.CSRFToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("X-CSRF-Token")),
		[]byte(sess.CSRFToken),
	) == 1
}
