package httpapi

import (
	"net/http"

	"github.com/ruhuang/ink/server/internal/pluginfetch"
	"github.com/ruhuang/ink/server/internal/plugins"
)

type installPluginFromGitRequest struct {
	RepoURL    string `json:"repoUrl"`
	RepoRef    string `json:"repoRef"`
	RepoSubdir string `json:"repoSubdir"`
}

func (s *Server) handleListAdminPlugins(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	items, err := s.plugins.ListAdminInstallations(r.Context(), accessToken)
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]plugins.PluginDetails{"plugins": items})
}

func (s *Server) handleUploadPlugin(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.pluginUploadMaxBytes)
	if err := r.ParseMultipartForm(pluginUploadMultipartMemory); err != nil {
		writeError(w, requestID, http.StatusBadRequest, "invalid_upload", "插件上传包无效或体积过大。")
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, requestID, http.StatusBadRequest, "missing_file", "请上传 ZIP 插件包。")
		return
	}
	defer func() { _ = file.Close() }()

	details, err := s.plugins.UploadPlugin(r.Context(), accessToken, header.Filename, file)
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]plugins.PluginDetails{"plugin": details})
}

func (s *Server) handleInstallPluginFromGit(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var req installPluginFromGitRequest
	if !decodeJSON(w, r, requestID, &req, defaultJSONMaxBytes, false) {
		return
	}

	details, err := s.plugins.InstallFromGit(r.Context(), accessToken, plugins.GitInstallInput{
		RepoURL: req.RepoURL,
		Ref:     req.RepoRef,
		Subdir:  req.RepoSubdir,
	})
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]plugins.PluginDetails{"plugin": details})
}

func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	details, err := s.plugins.DisableInstallation(r.Context(), accessToken, r.PathValue("installationID"))
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]plugins.PluginDetails{"plugin": details})
}

func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	items, err := s.plugins.ListUserPlugins(r.Context(), accessToken)
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]plugins.PluginDetails{"plugins": items})
}

func (s *Server) handleGetPlugin(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	details, err := s.plugins.GetUserPlugin(r.Context(), accessToken, r.PathValue("installationID"))
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]plugins.PluginDetails{"plugin": details})
}

func (s *Server) handleSavePluginBinding(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload pluginBindingRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	details, err := s.plugins.SaveBinding(r.Context(), accessToken, r.PathValue("installationID"), plugins.BindingInput{
		Enabled: payload.Enabled,
		Config:  payload.Config,
		Secrets: payload.Secrets,
	})
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]plugins.PluginDetails{"plugin": details})
}

func (s *Server) handleTestPluginBinding(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload pluginBindingRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	result, err := s.plugins.TestBinding(r.Context(), accessToken, r.PathValue("installationID"), plugins.BindingInput{
		Enabled: payload.Enabled,
		Config:  payload.Config,
		Secrets: payload.Secrets,
	})
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]plugins.ValidationResult{"result": result})
}

func (s *Server) handleRunPlugin(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload struct{}
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, true) {
		return
	}

	result, err := s.pluginRuns.RunManual(r.Context(), accessToken, r.PathValue("installationID"))
	if err != nil {
		s.writePluginError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]pluginfetch.ManualRunResult{"result": result})
}
