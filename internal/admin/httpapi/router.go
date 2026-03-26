package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/StevenBuglione/open-cli/internal/admin/authn"
	"github.com/StevenBuglione/open-cli/internal/admin/domain"
	"github.com/StevenBuglione/open-cli/internal/admin/service"
)

type Dependencies struct {
	Service       *service.Service
	TokenVerifier authn.TokenVerifier
}

func NewDependencies(svc *service.Service, verifier authn.TokenVerifier) Dependencies {
	return Dependencies{
		Service:       svc,
		TokenVerifier: verifier,
	}
}

func RegisterRoutes(mux *http.ServeMux, deps Dependencies) http.Handler {
	middleware := authn.NewMiddleware(deps.TokenVerifier)

	// Admin identity endpoint
	mux.Handle("/v1/admin/me", middleware.RequireAdmin(http.HandlerFunc(handleAdminMe)))

	// Bundle CRUD endpoints
	mux.Handle("/v1/admin/bundles", middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListBundles(w, r, deps)
		case http.MethodPost:
			handleCreateBundle(w, r, deps)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	mux.Handle("/v1/admin/bundles/", middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bundleID := strings.TrimPrefix(r.URL.Path, "/v1/admin/bundles/")
		
		// Check if it's an assignment operation
		if strings.HasSuffix(bundleID, "/assignments") {
			bundleID = strings.TrimSuffix(bundleID, "/assignments")
			switch r.Method {
			case http.MethodGet:
				handleListBundleAssignments(w, r, deps, bundleID)
			case http.MethodPost:
				handleCreateBundleAssignment(w, r, deps, bundleID)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleGetBundle(w, r, deps, bundleID)
		case http.MethodPut:
			handleUpdateBundle(w, r, deps, bundleID)
		case http.MethodDelete:
			handleDeleteBundle(w, r, deps, bundleID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Assignment deletion endpoint
	mux.Handle("/v1/admin/assignments/", middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		assignmentID := strings.TrimPrefix(r.URL.Path, "/v1/admin/assignments/")
		handleDeleteBundleAssignment(w, r, deps, assignmentID)
	})))

	// Source CRUD endpoints
	mux.Handle("/v1/admin/sources", middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListSources(w, r, deps)
		case http.MethodPost:
			handleCreateSource(w, r, deps)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	mux.Handle("/v1/admin/sources/", middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceID := strings.TrimPrefix(r.URL.Path, "/v1/admin/sources/")
		
		// Check if it's a validate operation
		if strings.HasSuffix(sourceID, "/validate") {
			sourceID = strings.TrimSuffix(sourceID, "/validate")
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			handleValidateSource(w, r, deps, sourceID)
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleGetSource(w, r, deps, sourceID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	return mux
}

func handleAdminMe(w http.ResponseWriter, r *http.Request) {
	identity := authn.GetIdentity(r.Context())
	if identity == nil {
		http.Error(w, "internal error: no identity in context", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(identity); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func handleCreateBundle(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	var input domain.CreateBundleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := deps.Service.CreateBundle(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func handleGetBundle(w http.ResponseWriter, r *http.Request, deps Dependencies, bundleID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	bundle, err := deps.Service.GetBundle(r.Context(), bundleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func handleListBundles(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	bundles, err := deps.Service.ListBundles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundles)
}

func handleUpdateBundle(w http.ResponseWriter, r *http.Request, deps Dependencies, bundleID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	var input domain.UpdateBundleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := deps.Service.UpdateBundle(r.Context(), bundleID, input); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleDeleteBundle(w http.ResponseWriter, r *http.Request, deps Dependencies, bundleID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	if err := deps.Service.DeleteBundle(r.Context(), bundleID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleCreateBundleAssignment(w http.ResponseWriter, r *http.Request, deps Dependencies, bundleID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	var input struct {
		PrincipalType string `json:"principal_type"`
		PrincipalID   string `json:"principal_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := deps.Service.CreateBundleAssignment(r.Context(), domain.CreateBundleAssignmentInput{
		BundleID:      bundleID,
		PrincipalType: input.PrincipalType,
		PrincipalID:   input.PrincipalID,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func handleListBundleAssignments(w http.ResponseWriter, r *http.Request, deps Dependencies, bundleID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	assignments, err := deps.Service.ListBundleAssignments(r.Context(), bundleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assignments)
}

func handleDeleteBundleAssignment(w http.ResponseWriter, r *http.Request, deps Dependencies, assignmentID string) {
	if deps.Service == nil {
		http.Error(w, "service not configured", http.StatusInternalServerError)
		return
	}

	if err := deps.Service.DeleteBundleAssignment(r.Context(), assignmentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleCreateSource(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	var input domain.CreateSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	source, err := deps.Service.CreateSource(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(source)
}

func handleGetSource(w http.ResponseWriter, r *http.Request, deps Dependencies, sourceID string) {
	source, err := deps.Service.GetSource(r.Context(), sourceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(source)
}

func handleListSources(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	sources, err := deps.Service.ListSources(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sources)
}

func handleValidateSource(w http.ResponseWriter, r *http.Request, deps Dependencies, sourceID string) {
	result, err := deps.Service.ValidateSource(r.Context(), sourceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
