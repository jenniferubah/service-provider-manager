.PHONY: build run clean fmt vet test test-coverage tidy generate-types generate-spec generate-server generate-client generate-api check-aep check-generate-api

BINARY_NAME := service-provider-manager

build:
	go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

run:
	go run ./cmd/$(BINARY_NAME)

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --fail-on-pending

tidy:
	go mod tidy

generate-types:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/types.gen.cfg \
		-o api/v1alpha1/types.gen.go \
		api/v1alpha1/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/resource_manager/types.gen.cfg \
		-o api/v1alpha1/resource_manager/types.gen.go \
		api/v1alpha1/resource_manager/openapi.yaml

generate-spec:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/spec.gen.cfg \
		-o api/v1alpha1/spec.gen.go \
		api/v1alpha1/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/resource_manager/spec.gen.cfg \
		-o api/v1alpha1/resource_manager/spec.gen.go \
		api/v1alpha1/resource_manager/openapi.yaml
generate-server:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/server/server.gen.cfg \
		-o internal/api/server/server.gen.go \
		api/v1alpha1/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/server/resource_manager/server.gen.cfg \
		-o internal/api/server/resource_manager/server.gen.go \
		api/v1alpha1/resource_manager/openapi.yaml
generate-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/client/client.gen.cfg \
		-o pkg/client/client.gen.go \
		api/v1alpha1/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/client/resource_manager/client.gen.cfg \
		-o pkg/client/resource_manager/client.gen.go \
		api/v1alpha1/resource_manager/openapi.yaml

generate-api: generate-types generate-spec generate-server generate-client

check-generate-api: generate-api
	git diff --exit-code api/ internal/api/server/ pkg/client/ || \
		(echo "Generated files out of sync. Run 'make generate-api'." && exit 1)

# Check AEP compliance
check-aep:
	spectral lint --fail-severity=warn ./api/v1alpha1/openapi.yaml
	spectral lint --fail-severity=warn ./api/v1alpha1/resource_manager/openapi.yaml

COVER_PKGS := ./internal/store/...,./internal/config/...,./internal/api_server/...

test-coverage:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --cover --coverpkg=$(COVER_PKGS) --coverprofile=coverage.out
	go tool cover -func=coverage.out

