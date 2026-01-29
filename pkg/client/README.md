# Service Provider Manager Client

Go client library for the Service Provider Manager API, generated from OpenAPI.

## Installation

```bash
go get github.com/dcm-project/service-provider-manager/pkg/client
```

## Usage

### Creating a Client

```go
import (
    "context"
    "github.com/dcm-project/service-provider-manager/api/v1alpha1"
    "github.com/dcm-project/service-provider-manager/pkg/client"
)

c, err := client.NewClientWithResponses("http://localhost:8080/api/v1alpha1")
if err != nil {
    log.Fatal(err)
}
```

### Registering a Service Provider

Service Providers register with DCM by calling the CreateProvider endpoint.
Registration is idempotent: if a provider with the same name exists, it will
be updated.

```go
ctx := context.Background()

resp, err := c.CreateProviderWithResponse(ctx, nil, v1alpha1.Provider{
    Name:          "my-kubevirt-provider",
    Endpoint:      "https://my-provider.local/api/v1",
    ServiceType:   "vm",
    SchemaVersion: "v1alpha1",
})
if err != nil {
    log.Fatalf("API call failed: %v", err)
}

switch resp.StatusCode() {
case 201:
    fmt.Printf("Registered new provider: %s\n", *resp.JSON201.Id)
case 200:
    fmt.Printf("Updated existing provider: %s\n", *resp.JSON200.Id)
case 409:
    fmt.Printf("Conflict: %s\n", resp.ApplicationproblemJSON409.Title)
case 400:
    fmt.Printf("Validation error: %s\n", resp.ApplicationproblemJSON400.Title)
default:
    log.Fatalf("Unexpected status: %d", resp.StatusCode())
}
```

### Registering with a Specific ID

Per AEP-133, specify the provider ID as a query parameter for idempotent
registration:

```go
import openapi_types "github.com/oapi-codegen/runtime/types"

providerID := openapi_types.UUID(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))
params := &v1alpha1.CreateProviderParams{Id: &providerID}

resp, err := c.CreateProviderWithResponse(ctx, params, v1alpha1.Provider{
    Name:          "my-provider",
    Endpoint:      "https://my-provider.local/api/v1",
    ServiceType:   "vm",
    SchemaVersion: "v1alpha1",
})
```

### Custom HTTP Client

Use a custom HTTP client for timeouts, TLS configuration, etc:

```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
}

c, err := client.NewClientWithResponses(
    "http://localhost:8080/api/v1alpha1",
    client.WithHTTPClient(httpClient),
)
```

## Error Handling

The API returns RFC 7807 Problem Details for errors. Check the response
status code and access the corresponding error field:

| Status | Field | Meaning |
|--------|-------|---------|
| 400 | `ApplicationproblemJSON400` | Invalid request |
| 404 | `ApplicationproblemJSON404` | Provider not found |
| 409 | `ApplicationproblemJSON409` | Conflict (name/ID exists) |
| 422 | `ApplicationproblemJSON422` | Unprocessable entity |

Example error handling:

```go
if resp.StatusCode() == 409 {
    apiErr := resp.ApplicationproblemJSON409
    fmt.Printf("Error: %s\n", apiErr.Title)
    if apiErr.Detail != nil {
        fmt.Printf("Detail: %s\n", *apiErr.Detail)
    }
}
```

## Other Operations

```go
// List providers
listResp, _ := c.ListProvidersWithResponse(ctx, nil)
for _, p := range *listResp.JSON200.Providers {
    fmt.Printf("- %s (%s)\n", p.Name, p.ServiceType)
}

// Get provider by ID
getResp, _ := c.GetProviderWithResponse(ctx, providerID)

// Update provider
updateResp, _ := c.ApplyProviderWithResponse(ctx, providerID, updatedProvider)

// Delete provider
deleteResp, _ := c.DeleteProviderWithResponse(ctx, providerID)
```
