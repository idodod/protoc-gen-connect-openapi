package converter

import (
	"log/slog"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	"github.com/sudorandom/protoc-gen-connect-openapi/internal/converter/options"
	"github.com/sudorandom/protoc-gen-connect-openapi/internal/converter/util"
)

func fileToComponents(opts options.Options, fd protoreflect.FileDescriptor) (*highv3.Components, error) {
	// Add schema from messages/enums
	components := &highv3.Components{
		Schemas:         orderedmap.New[string, *base.SchemaProxy](),
		Responses:       orderedmap.New[string, *highv3.Response](),
		Parameters:      orderedmap.New[string, *highv3.Parameter](),
		Examples:        orderedmap.New[string, *base.Example](),
		RequestBodies:   orderedmap.New[string, *highv3.RequestBody](),
		Headers:         orderedmap.New[string, *highv3.Header](),
		SecuritySchemes: orderedmap.New[string, *highv3.SecurityScheme](),
		Links:           orderedmap.New[string, *highv3.Link](),
		Callbacks:       orderedmap.New[string, *highv3.Callback](),
		Extensions:      orderedmap.New[string, *yaml.Node](),
	}
	st := NewState(opts)
	slog.Debug("start collection")
	st.CollectFile(fd)
	slog.Debug("collection complete", slog.String("file", string(fd.Name())), slog.Int("messages", len(st.Messages)), slog.Int("enum", len(st.Enums)))
	components.Schemas = stateToSchema(st)

	hasGetRequests := false
	hasMethods := false

	// Add requestBodies and responses for methods
	services := fd.Services()
	for i := 0; i < services.Len(); i++ {
		service := services.Get(i)
		methods := service.Methods()
		for j := 0; j < methods.Len(); j++ {
			method := methods.Get(j)
			hasGet := methodHasGet(opts, method)
			if hasGet {
				hasGetRequests = true
			}
			hasMethods = true
		}
	}

	if hasGetRequests {
		components.Schemas.Set("encoding", base.CreateSchemaProxy(&base.Schema{
			Title:       "encoding",
			Description: "Define which encoding or 'Message-Codec' to use",
			Enum: []*yaml.Node{
				utils.CreateStringNode("proto"),
				utils.CreateStringNode("json"),
			},
		}))

		components.Schemas.Set("base64", base.CreateSchemaProxy(&base.Schema{
			Title:       "base64",
			Description: "Specifies if the message query param is base64 encoded, which may be required for binary data",
			Type:        []string{"boolean"},
		}))

		components.Schemas.Set("compression", base.CreateSchemaProxy(&base.Schema{
			Title:       "compression",
			Description: "Which compression algorithm to use for this request",
			Enum: []*yaml.Node{
				utils.CreateStringNode("identity"),
				utils.CreateStringNode("gzip"),
				utils.CreateStringNode("br"),
			},
		}))
		components.Schemas.Set("connect", base.CreateSchemaProxy(&base.Schema{
			Title:       "connect",
			Description: "Define the version of the Connect protocol",
			Enum: []*yaml.Node{
				utils.CreateStringNode("v1"),
			},
		}))
	}
	if hasMethods {
		components.Schemas.Set("connect-protocol-version", base.CreateSchemaProxy(&base.Schema{
			Title:       "Connect-Protocol-Version",
			Description: "Define the version of the Connect protocol",
			Type:        []string{"number"},
			Enum:        []*yaml.Node{utils.CreateIntNode("1")},
			Const:       utils.CreateIntNode("1"),
		}))

		components.Schemas.Set("connect-timeout-header", base.CreateSchemaProxy(&base.Schema{
			Title:       "Connect-Timeout-Ms",
			Description: "Define the timeout, in ms",
			Type:        []string{"number"},
		}))

		connectErrorProps := orderedmap.New[string, *base.SchemaProxy]()
		connectErrorProps.Set("code", base.CreateSchemaProxy(&base.Schema{
			Description: "The status code, which should be an enum value of [google.rpc.Code][google.rpc.Code].",
			Type:        []string{"string"},
			Examples:    []*yaml.Node{utils.CreateStringNode("not_found")},
			Enum: []*yaml.Node{
				utils.CreateStringNode("canceled"),
				utils.CreateStringNode("unknown"),
				utils.CreateStringNode("invalid_argument"),
				utils.CreateStringNode("deadline_exceeded"),
				utils.CreateStringNode("not_found"),
				utils.CreateStringNode("already_exists"),
				utils.CreateStringNode("permission_denied"),
				utils.CreateStringNode("resource_exhausted"),
				utils.CreateStringNode("failed_precondition"),
				utils.CreateStringNode("aborted"),
				utils.CreateStringNode("out_of_range"),
				utils.CreateStringNode("unimplemented"),
				utils.CreateStringNode("internal"),
				utils.CreateStringNode("unavailable"),
				utils.CreateStringNode("data_loss"),
				utils.CreateStringNode("unauthenticated"),
			},
		}))
		connectErrorProps.Set("message", base.CreateSchemaProxy(&base.Schema{
			Description: "A developer-facing error message, which should be in English. Any user-facing error message should be localized and sent in the [google.rpc.Status.details][google.rpc.Status.details] field, or localized by the client.",
			Type:        []string{"string"},
		}))

		var detailSchemaProxy *base.SchemaProxy
		if opts.OverrideConnectErrorDetail {
			addConnectErrorDetailSchemas(components)
			detailSchemaProxy = base.CreateSchemaProxy(&base.Schema{
				Description: "A list of messages that carry the error details. There is a common set of message types for APIs to use.",
				Type:        []string{"array"},
				OneOf: []*base.SchemaProxy{
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.DebugInfo"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.Help"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.LocalizedMessage"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.RequestInfo"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.ResourceInfo"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.RetryInfo"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.QuotaFailure"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.PreconditionFailure"),
					base.CreateSchemaProxyRef("#/components/schemas/google.rpc.BadRequest"),
				},
			})
		} else {
			detailSchemaProxy = base.CreateSchemaProxyRef("#/components/schemas/google.protobuf.Any")
		}
		connectErrorProps.Set("detail", detailSchemaProxy)
		components.Schemas.Set("connect.error", base.CreateSchemaProxy(&base.Schema{
			Title:                "Connect Error",
			Description:          `Error type returned by Connect: https://connectrpc.com/docs/go/errors/#http-representation`,
			Properties:           connectErrorProps,
			Type:                 []string{"object"},
			AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{N: 1, B: true},
		}))
		anyPair := util.NewGoogleAny()
		components.Schemas.Set(anyPair.ID, base.CreateSchemaProxy(anyPair.Schema))
	}

	return components, nil
}

func addConnectErrorDetailSchemas(components *highv3.Components) {
	errorInfoProps := orderedmap.New[string, *base.SchemaProxy]()
	errorInfoProps.Set("reason", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The reason of the error in UPPER_SNAKE_CASE.",
	}))
	errorInfoProps.Set("domain", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The logical grouping to which the error reason belongs.",
	}))
	errorInfoProps.Set("metadata", base.CreateSchemaProxy(&base.Schema{
		Type:                 []string{"object"},
		AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}})},
		Description:          "Additional structured details about the error.",
	}))
	components.Schemas.Set("google.rpc.ErrorInfo", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: errorInfoProps,
	}))

	durationProps := orderedmap.New[string, *base.SchemaProxy]()
	durationProps.Set("seconds", base.CreateSchemaProxy(&base.Schema{
		Type:   []string{"integer"},
		Format: "int64",
	}))
	durationProps.Set("nanos", base.CreateSchemaProxy(&base.Schema{
		Type:   []string{"integer"},
		Format: "int32",
	}))
	components.Schemas.Set("google.rpc.Duration", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: durationProps,
	}))

	retryInfoProps := orderedmap.New[string, *base.SchemaProxy]()
	retryInfoProps.Set("retry_delay", base.CreateSchemaProxyRef("#/components/schemas/google.rpc.Duration"))
	components.Schemas.Set("google.rpc.RetryInfo", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: retryInfoProps,
	}))

	debugInfoProps := orderedmap.New[string, *base.SchemaProxy]()
	debugInfoProps.Set("stack_entries", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"array"},
		Items:       &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}})},
		Description: "The stack trace entries of the caller that led to the error being generated.",
	}))
	debugInfoProps.Set("detail", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "Additional debugging information provided by the server.",
	}))
	components.Schemas.Set("google.rpc.DebugInfo", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: debugInfoProps,
	}))

	quotaViolation := orderedmap.New[string, *base.SchemaProxy]()
	quotaViolation.Set("subject", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The subject on which the quota check failed.",
	}))
	quotaViolation.Set("description", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "A description of how the quota check failed.",
	}))
	components.Schemas.Set("google.rpc.QuotaFailure.Violation", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: quotaViolation,
	}))

	quotaFailure := orderedmap.New[string, *base.SchemaProxy]()
	quotaFailure.Set("violations", base.CreateSchemaProxy(&base.Schema{
		Type:  []string{"array"},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxyRef("#/components/schemas/google.rpc.QuotaFailure.Violation")},
	}))
	components.Schemas.Set("google.rpc.QuotaFailure", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: quotaFailure,
	}))

	preconditionViolations := orderedmap.New[string, *base.SchemaProxy]()
	preconditionViolations.Set("type", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The type of precondition failure.",
	}))
	preconditionViolations.Set("subject", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The subject relative to the type.",
	}))
	preconditionViolations.Set("description", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "A description of how the precondition failed.",
	}))
	components.Schemas.Set("google.rpc.PreconditionFailure.Violation", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: preconditionViolations,
	}))

	preconditionFailure := orderedmap.New[string, *base.SchemaProxy]()
	preconditionFailure.Set("violations", base.CreateSchemaProxy(&base.Schema{
		Type:  []string{"array"},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxyRef("#/components/schemas/google.rpc.PreconditionFailure.Violation")},
	}))
	components.Schemas.Set("google.rpc.PreconditionFailure", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: preconditionFailure,
	}))

	fieldViolations := orderedmap.New[string, *base.SchemaProxy]()
	fieldViolations.Set("field", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "A path that leads to a field in the request body.",
	}))
	fieldViolations.Set("description", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "A description of why the request element is invalid.",
	}))
	fieldViolations.Set("reason", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The reason for the field-level error in UPPER_SNAKE_CASE.",
	}))
	components.Schemas.Set("google.rpc.BadRequest.FieldViolation", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: fieldViolations,
	}))

	badRequest := orderedmap.New[string, *base.SchemaProxy]()
	badRequest.Set("field_violations", base.CreateSchemaProxy(&base.Schema{
		Type:  []string{"array"},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxyRef("#/components/schemas/google.rpc.BadRequest.FieldViolation")},
	}))
	components.Schemas.Set("google.rpc.BadRequest", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: badRequest,
	}))

	requestInfo := orderedmap.New[string, *base.SchemaProxy]()
	requestInfo.Set("request_id", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "An opaque string used for identifying requests in logs.",
	}))
	requestInfo.Set("serving_data", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "Data used to serve this request.",
	}))
	components.Schemas.Set("google.rpc.RequestInfo", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: requestInfo,
	}))

	resourceInfo := orderedmap.New[string, *base.SchemaProxy]()
	resourceInfo.Set("resource_type", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "A name for the type of resource being accessed.",
	}))
	resourceInfo.Set("resource_name", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The name of the resource being accessed.",
	}))
	resourceInfo.Set("owner", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The owner of the resource.",
	}))
	resourceInfo.Set("description", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "Describes what error is encountered when accessing this resource.",
	}))
	components.Schemas.Set("google.rpc.ResourceInfo", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: resourceInfo,
	}))

	helpLink := orderedmap.New[string, *base.SchemaProxy]()
	helpLink.Set("url", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The URL of the link.",
	}))
	helpLink.Set("description", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "Describes what the link offers.",
	}))
	components.Schemas.Set("google.rpc.Help", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: helpLink,
	}))

	localizedMessage := orderedmap.New[string, *base.SchemaProxy]()
	localizedMessage.Set("locale", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The locale used for the message.",
	}))
	localizedMessage.Set("message", base.CreateSchemaProxy(&base.Schema{
		Type:        []string{"string"},
		Description: "The localized error message.",
	}))
	components.Schemas.Set("google.rpc.LocalizedMessage", base.CreateSchemaProxy(&base.Schema{
		Type:       []string{"object"},
		Properties: localizedMessage,
	}))
}
