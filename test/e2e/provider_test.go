//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"os"

	"github.com/dcm-project/service-provider-manager/api/v1alpha1"
	"github.com/dcm-project/service-provider-manager/pkg/client"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Provider API", func() {
	var (
		apiClient *client.ClientWithResponses
		ctx       context.Context
	)

	BeforeEach(func() {
		baseURL := os.Getenv("API_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080/api/v1alpha1"
		}

		var err error
		apiClient, err = client.NewClientWithResponses(baseURL)
		Expect(err).NotTo(HaveOccurred())

		ctx = context.Background()
	})

	Describe("Health", func() {
		It("returns healthy status", func() {
			resp, err := apiClient.GetHealthWithResponse(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			Expect(resp.JSON200).NotTo(BeNil())
			Expect(*resp.JSON200.Status).To(Equal("ok"))
		})
	})

	Describe("Provider CRUD", func() {
		It("creates, reads, updates, and deletes a provider", func() {
			By("creating a new provider")
			createResp, err := apiClient.CreateProviderWithResponse(ctx, nil, v1alpha1.Provider{
				Name:          "e2e-test-provider",
				Endpoint:      "https://example.com/api",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))
			Expect(createResp.JSON201).NotTo(BeNil())
			Expect(createResp.JSON201.Id).NotTo(BeNil())
			Expect(*createResp.JSON201.Status).To(Equal(v1alpha1.Registered))

			providerID := *createResp.JSON201.Id

			By("getting the provider")
			getResp, err := apiClient.GetProviderWithResponse(ctx, providerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(getResp.JSON200.Name).To(Equal("e2e-test-provider"))

			By("re-registering without ID (idempotent update)")
			reregResp, err := apiClient.CreateProviderWithResponse(ctx, nil, v1alpha1.Provider{
				Name:          "e2e-test-provider",
				Endpoint:      "https://updated.example.com/api",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(reregResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(reregResp.JSON200).NotTo(BeNil())
			Expect(*reregResp.JSON200.Status).To(Equal(v1alpha1.Updated))
			Expect(reregResp.JSON200.Id.String()).To(Equal(providerID.String()))

			By("listing providers")
			listResp, err := apiClient.ListProvidersWithResponse(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(listResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(listResp.JSON200.Providers).NotTo(BeNil())
			Expect(len(*listResp.JSON200.Providers)).To(BeNumerically(">=", 1))

			By("updating the provider")
			updateResp, err := apiClient.ApplyProviderWithResponse(ctx, providerID, v1alpha1.Provider{
				Name:          "e2e-test-provider-updated",
				Endpoint:      "https://updated.example.com/api",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updateResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(updateResp.JSON200.Name).To(Equal("e2e-test-provider-updated"))

			By("deleting the provider")
			deleteResp, err := apiClient.DeleteProviderWithResponse(ctx, providerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteResp.StatusCode()).To(Equal(http.StatusNoContent))

			By("verifying provider is deleted")
			getDeletedResp, err := apiClient.GetProviderWithResponse(ctx, providerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(getDeletedResp.StatusCode()).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Conflict scenarios", func() {
		var providerID openapi_types.UUID

		BeforeEach(func() {
			resp, err := apiClient.CreateProviderWithResponse(ctx, nil, v1alpha1.Provider{
				Name:          "conflict-test-provider",
				Endpoint:      "https://example.com/api",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			providerID = *resp.JSON201.Id
		})

		AfterEach(func() {
			apiClient.DeleteProviderWithResponse(ctx, providerID)
		})

		It("returns 409 when registering same name with different ID", func() {
			newID := openapi_types.UUID(uuid.New())
			params := &v1alpha1.CreateProviderParams{Id: &newID}

			resp, err := apiClient.CreateProviderWithResponse(ctx, params, v1alpha1.Provider{
				Name:          "conflict-test-provider",
				Endpoint:      "https://other.example.com/api",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusConflict))
		})
	})
})
