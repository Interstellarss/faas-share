package server

import (
	"encoding/json"
	"net/http"

	"github.com/Interstellarss/faas-share/version"
	"github.com/openfaas/faas-provider/types"
	"k8s.io/klog"
)

func makeInfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
		}

		sha, release := version.GetReleaseInfo()

		info := types.ProviderInfo{
			Orchestration: "kubernetes",
			Name:          "faasshare-operator",
			Version: &types.VersionInfo{
				SHA:     sha,
				Release: release,
			},
		}

		infoBytes, err := json.Marshal(info)

		if err != nil {
			klog.Errorf("Failes to marshal info: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to marshal info"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(infoBytes)
	}
}
