// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"reflect"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func (api *API) InitConfigLocal() {
	api.BaseRoutes.ApiRoot.Handle("/config", api.ApiLocal(getConfig)).Methods("GET")
	api.BaseRoutes.ApiRoot.Handle("/config", api.ApiLocal(localUpdateConfig)).Methods("PUT")
	api.BaseRoutes.ApiRoot.Handle("/config/patch", api.ApiLocal(localPatchConfig)).Methods("PUT")
}

func localUpdateConfig(c *Context, w http.ResponseWriter, r *http.Request) {
	cfg := model.ConfigFromJson(r.Body)
	if cfg == nil {
		c.SetInvalidParam("config")
		return
	}

	auditRec := c.MakeAuditRecord("localUpdateConfig", audit.Fail)
	defer c.LogAuditRec(auditRec)

	cfg.SetDefaults()

	appCfg := c.App.Config()

	// Do not allow plugin uploads to be toggled through the API
	cfg.PluginSettings.EnableUploads = appCfg.PluginSettings.EnableUploads

	// Do not allow certificates to be changed through the API
	cfg.PluginSettings.SignaturePublicKeyFiles = appCfg.PluginSettings.SignaturePublicKeyFiles

	c.App.HandleMessageExportConfig(cfg, appCfg)

	err := cfg.IsValid()
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.SaveConfig(cfg, true)
	if err != nil {
		c.Err = err
		return
	}

	cfg = c.App.GetSanitizedConfig()

	auditRec.Success()
	c.LogAudit("updateConfig")

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(cfg.ToJson()))
}

func localPatchConfig(c *Context, w http.ResponseWriter, r *http.Request) {
	cfg := model.ConfigFromJson(r.Body)
	if cfg == nil {
		c.SetInvalidParam("config")
		return
	}

	auditRec := c.MakeAuditRecord("localPatchConfig", audit.Fail)
	defer c.LogAuditRec(auditRec)

	appCfg := c.App.Config()
	filterFn := func(structField reflect.StructField, base, patch reflect.Value) bool {
		return true
	}

	// Do not allow plugin uploads to be toggled through the API
	cfg.PluginSettings.EnableUploads = appCfg.PluginSettings.EnableUploads

	if cfg.MessageExportSettings.EnableExport != nil {
		c.App.HandleMessageExportConfig(cfg, appCfg)
	}

	updatedCfg, mergeErr := config.Merge(appCfg, cfg, &utils.MergeConfig{
		StructFieldFilter: filterFn,
	})

	if mergeErr != nil {
		c.Err = model.NewAppError("patchConfig", "api.config.update_config.restricted_merge.app_error", nil, mergeErr.Error(), http.StatusInternalServerError)
		return
	}

	err := updatedCfg.IsValid()
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.SaveConfig(updatedCfg, true)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(c.App.GetSanitizedConfig().ToJson()))
}
