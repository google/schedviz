//
// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS-IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Package main contains a simple web server for serving whitelisted static content.
package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"

	"github.com/google/schedviz/server/apiservice"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/server/storageservice"

	"flag"
	log "github.com/golang/glog"

)

var (
	port         = flag.Int("port", 7402, "The SchedViz HTTP port.")
	resourceRoot = flag.String("resources_root", "client", "The folder where the static files are stored.")
	storagePath  = flag.String("storage_path", "", "The folder where trace data is/will be stored.")
	cacheSize    = flag.Int("cache_size", 25, "The maximum number of collections to keep open at once.")
)


const (
	err404 = "Failed to fetch requested resource: %s"
	err500 = "Internal Server Error"
)

// Tag for serialized proto requests in JSON.
const requestTag = "request"
const fileTag = "file"

var storageService storageservice.StorageService


var defaultHTTPUser = "local_user"

var httpUser = func(w http.ResponseWriter, req *http.Request) (string, error) {
	return defaultHTTPUser, nil
}

var handle = func(r *mux.Router, path string, handler http.HandlerFunc) {
	r.HandleFunc(path, handler)
}

func newStaticHandler(resourceRoot string, privatePath string, contentType string) http.HandlerFunc {
	internalPath := path.Join(resourceRoot, privatePath)
	return func(w http.ResponseWriter, req *http.Request) {
		fileContent, err := ioutil.ReadFile(internalPath)
		if err != nil {
			http.Error(w, fmt.Sprintf(err404, req.URL.Path), http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", contentType)
		_, err = fmt.Fprintf(w, "%s", fileContent)
		if err != nil {
			http.Error(w, err500, http.StatusInternalServerError)
		}
	}
}

// redirectHandler redirects to /dashboard
func redirectHandler(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/dashboard", http.StatusMovedPermanently)
}

func registerStaticHandlers(r *mux.Router, resourceRoot string) {
	htmlHandler := newStaticHandler(resourceRoot, "sched.html", "text/html")
	faviconHandler := newStaticHandler(resourceRoot, "favicon.ico", "image/x-icon")
	jsHandler := newStaticHandler(resourceRoot, "bundle.js", "application/javascript")
	styleHandler := newStaticHandler(resourceRoot, "material-theme.css", "text/css")
	pakoHandler := newStaticHandler(resourceRoot, "pako.min.js", "application/javascript")

	handle(r, "/", htmlHandler)
	handle(r, "/sched.html", redirectHandler)
	handle(r, "/favicon.ico", faviconHandler)
	handle(r, "/bundle.js", jsHandler)
	handle(r, "/pako.min.js", pakoHandler)
	handle(r, "/material-theme", styleHandler)
}

type storageServiceHTTPHandler struct{ storageservice.StorageService }


func (s *storageServiceHTTPHandler) handleUpload(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	user, err := httpUser(w, req)
	if err != nil {
		http.Error(w, "Failed to get HTTP user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// 100 MB memory limit
	if err := req.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		log.Error(err)
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}

	jsonreq := &models.CreateCollectionRequest{}
	if err := json.Unmarshal([]byte(req.Form.Get(requestTag)), jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonreq.Creator = user

	file, err := req.MultipartForm.File[fileTag][0].Open()
	if err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("failed to close multipart temp file: %s", err)
		}
	}()

	collectionName, err := s.UploadFile(ctx, jsonreq, file)
	if err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	sendStringHTTPResponse(req, collectionName, w)
}

func (s *storageServiceHTTPHandler) handleGetCollectionMetadata(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}

	un := req.Form.Get(requestTag)
	md, err := s.GetCollectionMetadata(ctx, un)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get collection metadata: %s", err), http.StatusInternalServerError)
		return
	}
	if cmp.Equal(md, models.Metadata{}) {
		http.Error(w, fmt.Sprintf("Couldn't find metadata for collection %s", un), http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, md, w)
}

func (s *storageServiceHTTPHandler) handleDeleteCollection(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	user, err := httpUser(w, req)
	if err != nil {
		http.Error(w, "Failed to get HTTP user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	un := req.Form.Get(requestTag)
	if err := s.DeleteCollection(ctx, user, un); err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to delete collection: %s", err),
			http.StatusInternalServerError)
		return
	}
}

func (s *storageServiceHTTPHandler) handleEditCollection(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	user, err := httpUser(w, req)
	if err != nil {
		http.Error(w, "Failed to get HTTP user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.EditCollectionRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.EditCollection(ctx, user, jsonreq); err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to edit collection: %s", err),
			http.StatusInternalServerError)
		return
	}
}

func (s *storageServiceHTTPHandler) handleGetCollectionParameters(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	cn := req.Form.Get(requestTag)
	params, err := s.GetCollectionParameters(ctx, cn)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to get collection parameters: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, params, w)
}

func (s *storageServiceHTTPHandler) handleListCollectionMetadata(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	user, err := httpUser(w, req)
	if err != nil {
		http.Error(w, "Failed to get HTTP user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	cn := req.Form.Get(requestTag)
	res, err := s.ListCollectionMetadata(ctx, user, cn)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to list collection metadata: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (s *storageServiceHTTPHandler) handleGetFtraceEvents(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.FtraceEventsRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := s.GetFtraceEvents(ctx, jsonreq)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to get Ftrace events: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func registerStorageService(r *mux.Router, s storageservice.StorageService) {
	sh := &storageServiceHTTPHandler{s}
	handle(r, "/get_collection_metadata", sh.handleGetCollectionMetadata)
	handle(r, "/upload", sh.handleUpload)
	handle(r, "/delete_collection", sh.handleDeleteCollection)
	handle(r, "/edit_collection", sh.handleEditCollection)
	handle(r, "/get_collection_parameters", sh.handleGetCollectionParameters)
	handle(r, "/list_collection_metadata", sh.handleListCollectionMetadata)
	handle(r, "/get_ftrace_events", sh.handleGetFtraceEvents)
}

type apiServiceHTTPHandler struct{ *apiservice.APIService }

func (a *apiServiceHTTPHandler) handleGetCPUIntervals(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.CPUIntervalsRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetCPUIntervals(ctx, jsonreq)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to get cpu intervals: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleGetPIDIntervals(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.PidIntervalsRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetPIDIntervals(ctx, jsonreq)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to get pid intervals: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleGetAntagonists(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.AntagonistsRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetAntagonists(ctx, jsonreq)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to get antagonists: %s", err),
			http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleGetPerThreadEventSeries(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.PerThreadEventSeriesRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetPerThreadEventSeries(ctx, jsonreq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get per thread event series: %s", err), http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleGetThreadSummaries(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.ThreadSummariesRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetThreadSummaries(ctx, jsonreq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get per thread summaries: %s", err), http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleGetUtilizationMetrics(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	jsonreq := &models.UtilizationMetricsRequest{}
	if err := readRequestBodyIntoStruct(req, jsonreq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := a.GetUtilizationMetrics(ctx, jsonreq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get utilization metrics: %s", err), http.StatusInternalServerError)
		return
	}
	sendStructHTTPResponse(req, res, w)
}

func (a *apiServiceHTTPHandler) handleSystemTopology(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
		return
	}
	cn := req.Form.Get(requestTag)
	st, err := a.GetSystemTopology(ctx, cn)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system topology: %s", err), http.StatusInternalServerError)
		return
	}

	jsonresp := models.SystemTopologyResponse{
		CollectionName: cn,
		SystemTopology: st,
	}
	sendStructHTTPResponse(req, jsonresp, w)
}

func registerAPIService(r *mux.Router, a *apiservice.APIService) {
	ah := &apiServiceHTTPHandler{a}
	handle(r, "/get_cpu_intervals", ah.handleGetCPUIntervals)
	handle(r, "/get_pid_intervals", ah.handleGetPIDIntervals)
	handle(r, "/get_antagonists", ah.handleGetAntagonists)
	handle(r, "/get_per_thread_event_series", ah.handleGetPerThreadEventSeries)
	handle(r, "/get_thread_summaries", ah.handleGetThreadSummaries)
	handle(r, "/get_utilization_metrics", ah.handleGetUtilizationMetrics)
	handle(r, "/get_system_topology", ah.handleSystemTopology)
}


var startServer = func(r *mux.Router) {
		http.Handle("/", r)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
			log.Fatal(err)
		}
}


var setStorageService = func(ctx context.Context) error {
	var ss storageservice.StorageService
	var err error
		ss, err = storageservice.CreateFSStorage(*storagePath, *cacheSize)
	if err != nil {
		return err
	}
	storageService = ss
	return nil
}

func runServer(ctx context.Context) {
	var r = mux.NewRouter()
	if err := setStorageService(ctx); err != nil {
		log.Exit(err)
	}
	// END-INTERNAL

	apiService := &apiservice.APIService{StorageService: storageService}

	registerStaticHandlers(r, *resourceRoot)
	registerStorageService(r, storageService)
	registerAPIService(r, apiService)
	// This must be the last route. If it isn't, it will consume the others.
	handle(r, "/{.*}", newStaticHandler(*resourceRoot, "sched.html", "text/html"))
	startServer(r)
}

func main() {
		flag.Parse()
	runServer(context.Background())
}

// gzipEnabledWriter returns a gzip writer that wraps the http.ResponseWriter if the client supports
// reading gzip; if it does not, the http.ResponseWriter is returned unchanged.
// The function also returns a closing function. For gzip, this will be a real function that must be
// called before sending the request, for http.ResponseWriter, it will be a no-op.
func gzipEnabledWriter(req *http.Request, w http.ResponseWriter) (io.Writer, func() error) {
	if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		// If content-length was set before compression, it'll be wrong.
		w.Header().Del("Content-Length")
		gzw := gzip.NewWriter(w)
		return gzw, gzw.Close
	}
	return w, func() error { return nil }
}

func sendStringHTTPResponse(req *http.Request, res string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	writer, closer := gzipEnabledWriter(req, w)
	defer func() { _ = closer() }()
	if _, err := writer.Write([]byte(res)); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
	}
}

func sendStructHTTPResponse(req *http.Request, res interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	writer, closer := gzipEnabledWriter(req, w)
	defer func() { _ = closer() }()
	if err := json.NewEncoder(writer).Encode(res); err != nil {
		http.Error(w, err500, http.StatusInternalServerError)
	}
}

func checkRequestContentType(req *http.Request, contentType string) error {
	gotContentType := req.Header.Get("Content-Type")
	if gotContentType != contentType {
		return fmt.Errorf("unexpected content type. want: %s, got: %s", contentType, gotContentType)
	}
	return nil
}

func readRequestBodyIntoStruct(req *http.Request, s interface{}) error {
	if err := checkRequestContentType(req, "application/json"); err != nil {
		return err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %s", err)
	}
	if err := req.Body.Close(); err != nil {
		return fmt.Errorf("error closing response body: %s", err)
	}
	if err := json.Unmarshal(body, s); err != nil {
		return fmt.Errorf("failed to unmarshal response JSON: %s", err)
	}
	return nil
}
