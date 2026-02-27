package handlers

import (
	"net/http"

	"nusantara/internal/platform/oscheck"
)

type systemCompatibilityResponse struct {
	TargetOS          string         `json:"target_os"`
	SingleServerOnly  bool           `json:"single_server_only"`
	NativeDeployment  bool           `json:"native_on_server"`
	CompatibilityInfo oscheck.Result `json:"compatibility_info"`
}

func SystemCompatibility(result oscheck.Result) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, systemCompatibilityResponse{
			TargetOS:          "ubuntu 22.04+",
			SingleServerOnly:  true,
			NativeDeployment:  true,
			CompatibilityInfo: result,
		})
	})
}

