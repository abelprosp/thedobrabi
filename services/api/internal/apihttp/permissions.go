package apihttp

import (
	"net/http"

	"github.com/thedobra/thedobra/services/api/internal/httpx"
)

// The database keeps "owner" for backwards compatibility. In the product
// model it is an administrator with the same full-access permissions.
func isAdmin(role string) bool {
	return role == "owner" || role == "admin"
}

func canAnalyze(role string) bool {
	return isAdmin(role) || role == "analyst"
}

func requireAdmin(w http.ResponseWriter, role string) bool {
	if isAdmin(role) {
		return true
	}
	httpx.Error(w, 403, "forbidden", "esta ação está disponível apenas para administradores")
	return false
}

func requireAnalyst(w http.ResponseWriter, role string) bool {
	if canAnalyze(role) {
		return true
	}
	httpx.Error(w, 403, "forbidden", "esta ação está disponível para analistas ou administradores")
	return false
}
