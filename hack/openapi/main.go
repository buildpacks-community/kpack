package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/pivotal/kpack/pkg/openapi"
)

type CRD struct {
	Group   string
	Version string
	Kind    string
	Plural  string
	Ref     string
	ListRef string
}

func main() {
	k8sDefs := spec.Definitions{}
	if err := json.Unmarshal([]byte(k8sOpenAPIDefinitions), &k8sDefs); err != nil {
		log.Fatal(err)
	}

	k8sOpenAPISchemas := map[string]spec.Schema{}
	for name, def := range k8sDefs {
		k8sOpenAPISchemas[name] = def
	}

	kpackOpenAPIDefs := openapi.GetOpenAPIDefinitions(func(path string) spec.Ref {
		return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(fixName(path)))
	})

	combinedOpenAPIDefs := map[string]spec.Schema{}
	for name, schema := range k8sOpenAPISchemas {
		combinedOpenAPIDefs[name] = schema
	}
	for name, def := range kpackOpenAPIDefs {
		combinedOpenAPIDefs[fixName(name)] = def.Schema
	}

	funcMap := template.FuncMap{
		"Title": strings.Title,
	}
	namespaceTemplate := template.Must(template.New("namespaceCRDPathsTemplate").Funcs(funcMap).Parse(namespaceCRDPathsTemplate))
	clusterTemplate := template.Must(template.New("clusterCRDPathsTemplate").Funcs(funcMap).Parse(clusterCRDPathsTemplate))

	paths := combineCRDPaths(
		getCRDPaths(namespaceTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "Builder",
			Plural:  "builders",
			Ref:     "#/definitions/kpack.build.v1alpha1.Builder",
			ListRef: "#/definitions/kpack.build.v1alpha1.BuilderList",
		}),
		getCRDPaths(namespaceTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "Build",
			Plural:  "builds",
			Ref:     "#/definitions/kpack.build.v1alpha1.Build",
			ListRef: "#/definitions/kpack.build.v1alpha1.BuildList",
		}),
		getCRDPaths(namespaceTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "Image",
			Plural:  "images",
			Ref:     "#/definitions/kpack.build.v1alpha1.Image",
			ListRef: "#/definitions/kpack.build.v1alpha1.ImageList",
		}),
		getCRDPaths(namespaceTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "SourceResolver",
			Plural:  "sourceresolvers",
			Ref:     "#/definitions/kpack.build.v1alpha1.SourceResolver",
			ListRef: "#/definitions/kpack.build.v1alpha1.SourceResolverList",
		}),
		getCRDPaths(clusterTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "ClusterBuilder",
			Plural:  "clusterbuilders",
			Ref:     "#/definitions/kpack.build.v1alpha1.ClusterBuilder",
			ListRef: "#/definitions/kpack.build.v1alpha1.ClusterBuilderList",
		}),
		getCRDPaths(clusterTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "ClusterStack",
			Plural:  "clusterstacks",
			Ref:     "#/definitions/kpack.build.v1alpha1.ClusterStack",
			ListRef: "#/definitions/kpack.build.v1alpha1.ClusterStackList",
		}),
		getCRDPaths(clusterTemplate, CRD{
			Group:   "kpack.io",
			Version: "v1alpha1",
			Kind:    "ClusterStore",
			Plural:  "clusterstores",
			Ref:     "#/definitions/kpack.build.v1alpha1.ClusterStore",
			ListRef: "#/definitions/kpack.build.v1alpha1.ClusterStoreList",
		}),
	)

	openAPISpec := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:   "kpack",
					Version: "v0.1.3",
				},
			},
			Paths: &spec.Paths{
				Paths: paths,
			},
			Definitions: combinedOpenAPIDefs,
		},
	}

	buf2, err := json.MarshalIndent(openAPISpec, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(buf2))
}

func fixName(name string) string {
	name = strings.ReplaceAll(name, "/", ".")
	if strings.Contains(name, "github.com.pivotal") {
		return strings.ReplaceAll(name, "github.com.pivotal.kpack.pkg.apis", "kpack")
	}
	if strings.Contains(name, "k8s.io") {
		return strings.ReplaceAll(name, "k8s.io", "io.k8s")
	}
	return name
}

func getCRDPaths(t *template.Template, crd CRD) map[string]spec.PathItem {
	buf := &bytes.Buffer{}
	err := t.Execute(buf, crd)
	if err != nil {
		log.Fatal(err)
	}

	paths := map[string]spec.PathItem{}
	if err := json.Unmarshal(buf.Bytes(), &paths); err != nil {
		log.Fatal(err)
	}

	return paths
}

func combineCRDPaths(paths ...map[string]spec.PathItem) map[string]spec.PathItem {
	combined := map[string]spec.PathItem{}
	for _, m := range paths {
		for k, v := range m {
			combined[k] = v
		}
	}
	return combined
}

// The following are the kubernetes schema definitions required by kpack. They have been cherry-picked from: https://github.com/kubernetes/kubernetes/tree/master/api/openapi-spec
const k8sOpenAPIDefinitions = `{
  "io.k8s.apimachinery.pkg.apis.meta.v1.ListMeta": {
    "description": "ListMeta describes metadata that synthetic resources must have, including lists and various status objects. A resource may have only one of {ObjectMeta, ListMeta}.",
    "type": "object",
    "properties": {
      "continue": {
        "description": "continue may be set if the user set a limit on the number of items returned, and indicates that the server has more data available. The value is opaque and may be used to issue another request to the endpoint that served this list to retrieve the next set of available objects. Continuing a consistent list may not be possible if the server configuration has changed or more than a few minutes have passed. The resourceVersion field returned when using this continue value will be identical to the value in the first response, unless you have received this token from an error message.",
        "type": "string"
      },
      "resourceVersion": {
        "description": "String that identifies the server's internal version of this object that can be used by clients to determine when objects have changed. Value must be treated as opaque by clients and passed unmodified back to the server. Populated by the system. Read-only. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#concurrency-control-and-consistency",
        "type": "string"
      },
      "selfLink": {
        "description": "selfLink is a URL representing this object. Populated by the system. Read-only.",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": {
    "description": "ObjectMeta is metadata that all persisted resources must have, which includes all objects users must create.",
    "type": "object",
    "properties": {
      "annotations": {
        "description": "Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: http://kubernetes.io/docs/user-guide/annotations",
        "type": "object",
        "additionalProperties": {
          "type": "string"
        }
      },
      "clusterName": {
        "description": "The name of the cluster which the object belongs to. This is used to distinguish resources with same name and namespace in different clusters. This field is not set anywhere right now and apiserver is going to ignore it if set in create or update request.",
        "type": "string"
      },
      "creationTimestamp": {
        "description": "CreationTimestamp is a timestamp representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations. Clients may not set this value. It is represented in RFC3339 form and is in UTC.\n\nPopulated by the system. Read-only. Null for lists. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      },
      "deletionGracePeriodSeconds": {
        "description": "Number of seconds allowed for this object to gracefully terminate before it will be removed from the system. Only set when deletionTimestamp is also set. May only be shortened. Read-only.",
        "type": "integer",
        "format": "int64"
      },
      "deletionTimestamp": {
        "description": "DeletionTimestamp is RFC 3339 date and time at which this resource will be deleted. This field is set by the server when a graceful deletion is requested by the user, and is not directly settable by a client. The resource is expected to be deleted (no longer visible from resource lists, and not reachable by name) after the time in this field, once the finalizers list is empty. As long as the finalizers list contains items, deletion is blocked. Once the deletionTimestamp is set, this value may not be unset or be set further into the future, although it may be shortened or the resource may be deleted prior to this time. For example, a user may request that a pod is deleted in 30 seconds. The Kubelet will react by sending a graceful termination signal to the containers in the pod. After that 30 seconds, the Kubelet will send a hard termination signal (SIGKILL) to the container and after cleanup, remove the pod from the API. In the presence of network partitions, this object may still exist after this timestamp, until an administrator or automated process can determine the resource is fully terminated. If not set, graceful deletion of the object has not been requested.\n\nPopulated by the system when a graceful deletion is requested. Read-only. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      },
      "finalizers": {
        "description": "Must be empty before the object is deleted from the registry. Each entry is an identifier for the responsible component that will remove the entry from the list. If the deletionTimestamp of the object is non-nil, entries in this list can only be removed.",
        "type": "array",
        "items": {
          "type": "string"
        },
        "x-kubernetes-patch-strategy": "merge"
      },
      "generateName": {
        "description": "GenerateName is an optional prefix, used by the server, to generate a unique name ONLY IF the Name field has not been provided. If this field is used, the name returned to the client will be different than the name passed. This value will also be combined with a unique suffix. The provided value has the same validation rules as the Name field, and may be truncated by the length of the suffix required to make the value unique on the server.\n\nIf this field is specified and the generated name exists, the server will NOT return a 409 - instead, it will either return 201 Created or 500 with Reason ServerTimeout indicating a unique name could not be found in the time allotted, and the client should retry (optionally after the time indicated in the Retry-After header).\n\nApplied only if Name is not specified. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#idempotency",
        "type": "string"
      },
      "generation": {
        "description": "A sequence number representing a specific generation of the desired state. Populated by the system. Read-only.",
        "type": "integer",
        "format": "int64"
      },
      "initializers": {
        "description": "An initializer is a controller which enforces some system invariant at object creation time. This field is a list of initializers that have not yet acted on this object. If nil or empty, this object has been completely initialized. Otherwise, the object is considered uninitialized and is hidden (in list/watch and get calls) from clients that haven't explicitly asked to observe uninitialized objects.\n\nWhen an object is created, the system will populate this list with the current set of initializers. Only privileged users may set or modify this list. Once it is empty, it may not be modified further by any user.\n\nDEPRECATED - initializers are an alpha field and will be removed in v1.15.",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Initializers"
      },
      "labels": {
        "description": "Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: http://kubernetes.io/docs/user-guide/labels",
        "type": "object",
        "additionalProperties": {
          "type": "string"
        }
      },
      "managedFields": {
        "description": "ManagedFields maps workflow-id and version to the set of fields that are managed by that workflow. This is mostly for internal housekeeping, and users typically shouldn't need to set or understand this field. A workflow can be the user's name, a controller's name, or the name of a specific apply path like \"ci-cd\". The set of fields is always in the version that the workflow used when modifying the object.\n\nThis field is alpha and can be changed or removed without notice.",
        "type": "array",
        "items": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ManagedFieldsEntry"
        }
      },
      "name": {
        "description": "Name must be unique within a namespace. Is required when creating resources, although some resources may allow a client to request the generation of an appropriate name automatically. Name is primarily intended for creation idempotence and configuration definition. Cannot be updated. More info: http://kubernetes.io/docs/user-guide/identifiers#names",
        "type": "string"
      },
      "namespace": {
        "description": "Namespace defines the space within each name must be unique. An empty namespace is equivalent to the \"default\" namespace, but \"default\" is the canonical representation. Not all objects are required to be scoped to a namespace - the value of this field for those objects will be empty.\n\nMust be a DNS_LABEL. Cannot be updated. More info: http://kubernetes.io/docs/user-guide/namespaces",
        "type": "string"
      },
      "ownerReferences": {
        "description": "List of objects depended by this object. If ALL objects in the list have been deleted, this object will be garbage collected. If this object is managed by a controller, then an entry in this list will point to this controller, with the controller field set to true. There cannot be more than one managing controller.",
        "type": "array",
        "items": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference"
        },
        "x-kubernetes-patch-merge-key": "uid",
        "x-kubernetes-patch-strategy": "merge"
      },
      "resourceVersion": {
        "description": "An opaque value that represents the internal version of this object that can be used by clients to determine when objects have changed. May be used for optimistic concurrency, change detection, and the watch operation on a resource or set of resources. Clients must treat these values as opaque and passed unmodified back to the server. They may only be valid for a particular resource or set of resources.\n\nPopulated by the system. Read-only. Value must be treated as opaque by clients and . More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#concurrency-control-and-consistency",
        "type": "string"
      },
      "selfLink": {
        "description": "SelfLink is a URL representing this object. Populated by the system. Read-only.",
        "type": "string"
      },
      "uid": {
        "description": "UID is the unique in time and space value for this object. It is typically generated by the server on successful creation of a resource and is not allowed to change on PUT operations.\n\nPopulated by the system. Read-only. More info: http://kubernetes.io/docs/user-guide/identifiers#uids",
        "type": "string"
      }
    }
  },
  "io.k8s.api.core.v1.LocalObjectReference": {
    "description": "LocalObjectReference contains enough information to let you locate the referenced object inside the same namespace.",
    "type": "object",
    "properties": {
      "name": {
        "description": "Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Time": {
    "description": "Time is a wrapper around time.Time which supports correct marshaling to YAML and JSON.  Wrappers are provided for many of the factory methods that the time package offers.",
    "type": "string",
    "format": "date-time"
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Initializers": {
    "description": "Initializers tracks the progress of initialization.",
    "type": "object",
    "required": [
      "pending"
    ],
    "properties": {
      "pending": {
        "description": "Pending is a list of initializers that must execute in order before this object is visible. When the last pending initializer is removed, and no failing result is set, the initializers struct will be set to nil and the object is considered as initialized and visible to all clients.",
        "type": "array",
        "items": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Initializer"
        },
        "x-kubernetes-patch-merge-key": "name",
        "x-kubernetes-patch-strategy": "merge"
      },
      "result": {
        "description": "If result is set with the Failure field, the object will be persisted to storage and then deleted, ensuring that other clients can observe the deletion.",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Status"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.ManagedFieldsEntry": {
    "description": "ManagedFieldsEntry is a workflow-id, a FieldSet and the group version of the resource that the fieldset applies to.",
    "type": "object",
    "properties": {
      "apiVersion": {
        "description": "APIVersion defines the version of this resource that this field set applies to. The format is \"group/version\" just like the top-level APIVersion field. It is necessary to track the version of a field set because it cannot be automatically converted.",
        "type": "string"
      },
      "fields": {
        "description": "Fields identifies a set of fields.",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Fields"
      },
      "manager": {
        "description": "Manager is an identifier of the workflow managing these fields.",
        "type": "string"
      },
      "operation": {
        "description": "Operation is the type of operation which lead to this ManagedFieldsEntry being created. The only valid values for this field are 'Apply' and 'Update'.",
        "type": "string"
      },
      "time": {
        "description": "Time is timestamp of when these fields were set. It should always be empty if Operation is 'Apply'",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference": {
    "description": "OwnerReference contains enough information to let you identify an owning object. An owning object must be in the same namespace as the dependent, or be cluster-scoped, so there is no namespace field.",
    "type": "object",
    "required": [
      "apiVersion",
      "kind",
      "name",
      "uid"
    ],
    "properties": {
      "apiVersion": {
        "description": "API version of the referent.",
        "type": "string"
      },
      "blockOwnerDeletion": {
        "description": "If true, AND if the owner has the \"foregroundDeletion\" finalizer, then the owner cannot be deleted from the key-value store until this reference is removed. Defaults to false. To set this field, a user needs \"delete\" permission of the owner, otherwise 422 (Unprocessable Entity) will be returned.",
        "type": "boolean"
      },
      "controller": {
        "description": "If true, this reference points to the managing controller.",
        "type": "boolean"
      },
      "kind": {
        "description": "Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": "string"
      },
      "name": {
        "description": "Name of the referent. More info: http://kubernetes.io/docs/user-guide/identifiers#names",
        "type": "string"
      },
      "uid": {
        "description": "UID of the referent. More info: http://kubernetes.io/docs/user-guide/identifiers#uids",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Status": {
    "description": "Status is a return value for calls that don't return other objects.",
    "type": "object",
    "properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": "string"
      },
      "code": {
        "description": "Suggested HTTP return code for this status, 0 if not set.",
        "type": "integer",
        "format": "int32"
      },
      "details": {
        "description": "Extended data associated with the reason.  Each reason may define its own extended details. This field is optional and the data returned is not guaranteed to conform to any schema except that defined by the reason type.",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.StatusDetails"
      },
      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": "string"
      },
      "message": {
        "description": "A human-readable description of the status of this operation.",
        "type": "string"
      },
      "metadata": {
        "description": "Standard list metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ListMeta"
      },
      "reason": {
        "description": "A machine-readable description of why this operation is in the \"Failure\" status. If this value is empty there is no information available. A Reason clarifies an HTTP status code but does not override it.",
        "type": "string"
      },
      "status": {
        "description": "Status of the operation. One of: \"Success\" or \"Failure\". More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status",
        "type": "string"
      }
    },
    "x-kubernetes-group-version-kind": [
      {
        "group": "",
        "kind": "Status",
        "version": "v1"
      }
    ]
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Initializer": {
    "description": "Initializer is information about an initializer that has not yet completed.",
    "type": "object",
    "required": [
      "name"
    ],
    "properties": {
      "name": {
        "description": "name of the process that is responsible for initializing this object.",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Fields": {
    "description": "Fields stores a set of fields in a data structure like a Trie. To understand how this is used, see: https://github.com/kubernetes-sigs/structured-merge-diff",
    "type": "object"
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.StatusDetails": {
    "description": "StatusDetails is a set of additional properties that MAY be set by the server to provide additional information about a response. The Reason field of a Status object defines what attributes will be set. Clients must ignore fields that do not match the defined type of each attribute, and should assume that any attribute may be empty, invalid, or under defined.",
    "type": "object",
    "properties": {
      "causes": {
        "description": "The Causes array includes more details associated with the StatusReason failure. Not all StatusReasons may provide detailed causes.",
        "type": "array",
        "items": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.StatusCause"
        }
      },
      "group": {
        "description": "The group attribute of the resource associated with the status StatusReason.",
        "type": "string"
      },
      "kind": {
        "description": "The kind attribute of the resource associated with the status StatusReason. On some operations may differ from the requested resource Kind. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": "string"
      },
      "name": {
        "description": "The name attribute of the resource associated with the status StatusReason (when there is a single name which can be described).",
        "type": "string"
      },
      "retryAfterSeconds": {
        "description": "If specified, the time in seconds before the operation should be retried. Some errors may indicate the client must take an alternate action - for those errors this field may indicate how long to wait before taking the alternate action.",
        "type": "integer",
        "format": "int32"
      },
      "uid": {
        "description": "UID of the resource. (when there is a single resource which can be described). More info: http://kubernetes.io/docs/user-guide/identifiers#uids",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.StatusCause": {
    "description": "StatusCause provides more information about an api.Status failure, including cases when multiple errors are encountered.",
    "type": "object",
    "properties": {
      "field": {
        "description": "The field of the resource that has caused this error, as named by its JSON serialization. May include dot and postfix notation for nested attributes. Arrays are zero-indexed.  Fields may appear more than once in an array of causes due to fields having multiple errors. Optional.\n\nExamples:\n  \"name\" - the field \"name\" on the current resource\n  \"items[0].name\" - the field \"name\" on the first array entry in \"items\"",
        "type": "string"
      },
      "message": {
        "description": "A human-readable description of the cause of the error.  This field may be presented as-is to a reader.",
        "type": "string"
      },
      "reason": {
        "description": "A machine-readable description of the cause of the error. If this value is empty there is no information available.",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions": {
    "description": "DeleteOptions may be provided when deleting an API object.",
    "type": "object",
    "properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": "string"
      },
      "dryRun": {
        "description": "When present, indicates that modifications should not be persisted. An invalid or unrecognized dryRun directive will result in an error response and no further processing of the request. Valid values are: - All: all dry run stages will be processed",
        "type": "array",
        "items": {
          "type": "string"
        }
      },
      "gracePeriodSeconds": {
        "description": "The duration in seconds before the object should be deleted. Value must be non-negative integer. The value zero indicates delete immediately. If this value is nil, the default grace period for the specified type will be used. Defaults to a per object value if not specified. zero means delete immediately.",
        "type": "integer",
        "format": "int64"
      },
      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": "string"
      },
      "orphanDependents": {
        "description": "Deprecated: please use the PropagationPolicy, this field will be deprecated in 1.7. Should the dependent objects be orphaned. If true/false, the \"orphan\" finalizer will be added to/removed from the object's finalizers list. Either this field or PropagationPolicy may be set, but not both.",
        "type": "boolean"
      },
      "preconditions": {
        "description": "Must be fulfilled before a deletion is carried out. If not possible, a 409 Conflict status will be returned.",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Preconditions"
      },
      "propagationPolicy": {
        "description": "Whether and how garbage collection will be performed. Either this field or OrphanDependents may be set, but not both. The default policy is decided by the existing finalizer set in the metadata.finalizers and the resource-specific default policy. Acceptable values are: 'Orphan' - orphan the dependents; 'Background' - allow the garbage collector to delete the dependents in the background; 'Foreground' - a cascading policy that deletes all dependents in the foreground.",
        "type": "string"
      }
    },
    "x-kubernetes-group-version-kind": [
      {
        "group": "",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "admission.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "admissionregistration.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "apiextensions.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "apiregistration.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "apiregistration.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "apps",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "apps",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "apps",
        "kind": "DeleteOptions",
        "version": "v1beta2"
      },
      {
        "group": "auditregistration.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "authentication.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "authentication.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "authorization.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "authorization.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "autoscaling",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "autoscaling",
        "kind": "DeleteOptions",
        "version": "v2beta1"
      },
      {
        "group": "autoscaling",
        "kind": "DeleteOptions",
        "version": "v2beta2"
      },
      {
        "group": "batch",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "batch",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "batch",
        "kind": "DeleteOptions",
        "version": "v2alpha1"
      },
      {
        "group": "certificates.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "coordination.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "coordination.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "events.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "extensions",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "imagepolicy.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "networking.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "networking.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "node.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "node.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "policy",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "rbac.authorization.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "rbac.authorization.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "rbac.authorization.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "scheduling.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "scheduling.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "scheduling.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      },
      {
        "group": "settings.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "storage.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1"
      },
      {
        "group": "storage.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1alpha1"
      },
      {
        "group": "storage.k8s.io",
        "kind": "DeleteOptions",
        "version": "v1beta1"
      }
    ]
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Preconditions": {
    "description": "Preconditions must be fulfilled before an operation (update, delete, etc.) is carried out.",
    "type": "object",
    "properties": {
      "resourceVersion": {
        "description": "Specifies the target ResourceVersion",
        "type": "string"
      },
      "uid": {
        "description": "Specifies the target UID.",
        "type": "string"
      }
    }
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.Patch": {
    "description": "Patch is provided to give a concrete name and type to the Kubernetes PATCH request body.",
    "type": "object"
  },
  "io.k8s.api.core.v1.EnvVar": {
    "description": "EnvVar represents an environment variable present in a Container.",
    "type": "object",
    "required": [
      "name"
    ],
    "properties": {
      "name": {
        "description": "Name of the environment variable. Must be a C_IDENTIFIER.",
        "type": "string"
      },
      "value": {
        "description": "Variable references $(VAR_NAME) are expanded using the previous defined environment variables in the container and any service environment variables. If a variable cannot be resolved, the reference in the input string will be unchanged. The $(VAR_NAME) syntax can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded, regardless of whether the variable exists or not. Defaults to \"\".",
        "type": "string"
      },
      "valueFrom": {
        "description": "Source for the environment variable's value. Cannot be used if value is not empty.",
        "$ref": "#/definitions/io.k8s.api.core.v1.EnvVarSource"
      }
    }
  },
  "io.k8s.api.core.v1.EnvVarSource": {
    "description": "EnvVarSource represents a source for the value of an EnvVar.",
    "type": "object",
    "properties": {
      "configMapKeyRef": {
        "description": "Selects a key of a ConfigMap.",
        "$ref": "#/definitions/io.k8s.api.core.v1.ConfigMapKeySelector"
      },
      "fieldRef": {
        "description": "Selects a field of the pod: supports metadata.name, metadata.namespace, metadata.labels, metadata.annotations, spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP.",
        "$ref": "#/definitions/io.k8s.api.core.v1.ObjectFieldSelector"
      },
      "resourceFieldRef": {
        "description": "Selects a resource of the container: only resources limits and requests (limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.",
        "$ref": "#/definitions/io.k8s.api.core.v1.ResourceFieldSelector"
      },
      "secretKeyRef": {
        "description": "Selects a key of a secret in the pod's namespace",
        "$ref": "#/definitions/io.k8s.api.core.v1.SecretKeySelector"
      }
    }
  },
  "io.k8s.api.core.v1.ConfigMapKeySelector": {
    "description": "Selects a key from a ConfigMap.",
    "type": "object",
    "required": [
      "key"
    ],
    "properties": {
      "key": {
        "description": "The key to select.",
        "type": "string"
      },
      "name": {
        "description": "Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
        "type": "string"
      },
      "optional": {
        "description": "Specify whether the ConfigMap or it's key must be defined",
        "type": "boolean"
      }
    }
  },
  "io.k8s.api.core.v1.ObjectFieldSelector": {
    "description": "ObjectFieldSelector selects an APIVersioned field of an object.",
    "type": "object",
    "required": [
      "fieldPath"
    ],
    "properties": {
      "apiVersion": {
        "description": "Version of the schema the FieldPath is written in terms of, defaults to \"v1\".",
        "type": "string"
      },
      "fieldPath": {
        "description": "Path of the field to select in the specified API version.",
        "type": "string"
      }
    }
  },
  "io.k8s.api.core.v1.ResourceFieldSelector": {
    "description": "ResourceFieldSelector represents container resources (cpu, memory) and their output format",
    "type": "object",
    "required": [
      "resource"
    ],
    "properties": {
      "containerName": {
        "description": "Container name: required for volumes, optional for env vars",
        "type": "string"
      },
      "divisor": {
        "description": "Specifies the output format of the exposed resources, defaults to \"1\"",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.api.resource.Quantity"
      },
      "resource": {
        "description": "Required: resource to select",
        "type": "string"
      }
    }
  },
  "io.k8s.api.core.v1.SecretKeySelector": {
    "description": "SecretKeySelector selects a key of a Secret.",
    "type": "object",
    "required": [
      "key"
    ],
    "properties": {
      "key": {
        "description": "The key of the secret to select from.  Must be a valid secret key.",
        "type": "string"
      },
      "name": {
        "description": "Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
        "type": "string"
      },
      "optional": {
        "description": "Specify whether the Secret or it's key must be defined",
        "type": "boolean"
      }
    }
  },
  "io.k8s.api.core.v1.ResourceRequirements": {
    "description": "ResourceRequirements describes the compute resource requirements.",
    "type": "object",
    "properties": {
      "limits": {
        "description": "Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
        "type": "object",
        "additionalProperties": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.api.resource.Quantity"
        }
      },
      "requests": {
        "description": "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
        "type": "object",
        "additionalProperties": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.api.resource.Quantity"
        }
      }
    }
  },
  "io.k8s.apimachinery.pkg.api.resource.Quantity": {
    "description": "Quantity is a fixed-point representation of a number. It provides convenient marshaling/unmarshaling in JSON and YAML, in addition to String() and Int64() accessors.\n\nThe serialization format is:\n\n<quantity>        ::= <signedNumber><suffix>\n  (Note that <suffix> may be empty, from the \"\" case in <decimalSI>.)\n<digit>           ::= 0 | 1 | ... | 9 <digits>          ::= <digit> | <digit><digits> <number>          ::= <digits> | <digits>.<digits> | <digits>. | .<digits> <sign>            ::= \"+\" | \"-\" <signedNumber>    ::= <number> | <sign><number> <suffix>          ::= <binarySI> | <decimalExponent> | <decimalSI> <binarySI>        ::= Ki | Mi | Gi | Ti | Pi | Ei\n  (International System of units; See: http://physics.nist.gov/cuu/Units/binary.html)\n<decimalSI>       ::= m | \"\" | k | M | G | T | P | E\n  (Note that 1024 = 1Ki but 1000 = 1k; I didn't choose the capitalization.)\n<decimalExponent> ::= \"e\" <signedNumber> | \"E\" <signedNumber>\n\nNo matter which of the three exponent forms is used, no quantity may represent a number greater than 2^63-1 in magnitude, nor may it have more than 3 decimal places. Numbers larger or more precise will be capped or rounded up. (E.g.: 0.1m will rounded up to 1m.) This may be extended in the future if we require larger or smaller quantities.\n\nWhen a Quantity is parsed from a string, it will remember the type of suffix it had, and will use the same type again when it is serialized.\n\nBefore serializing, Quantity will be put in \"canonical form\". This means that Exponent/suffix will be adjusted up or down (with a corresponding increase or decrease in Mantissa) such that:\n  a. No precision is lost\n  b. No fractional digits will be emitted\n  c. The exponent (or suffix) is as large as possible.\nThe sign will be omitted unless the number is negative.\n\nExamples:\n  1.5 will be serialized as \"1500m\"\n  1.5Gi will be serialized as \"1536Mi\"\n\nNote that the quantity will NEVER be internally represented by a floating point number. That is the whole point of this exercise.\n\nNon-canonical values will still parse as long as they are well formed, but will be re-emitted in their canonical form. (So always use canonical form, or don't diff.)\n\nThis format is intended to make it difficult to use these numbers without writing some sort of special handling code in the hopes that that will cause implementors to also use a fixed point implementation.",
    "type": "string"
  },
  "io.k8s.api.core.v1.ContainerState": {
    "description": "ContainerState holds a possible state of container. Only one of its members may be specified. If none of them is specified, the default one is ContainerStateWaiting.",
    "type": "object",
    "properties": {
      "running": {
        "description": "Details about a running container",
        "$ref": "#/definitions/io.k8s.api.core.v1.ContainerStateRunning"
      },
      "terminated": {
        "description": "Details about a terminated container",
        "$ref": "#/definitions/io.k8s.api.core.v1.ContainerStateTerminated"
      },
      "waiting": {
        "description": "Details about a waiting container",
        "$ref": "#/definitions/io.k8s.api.core.v1.ContainerStateWaiting"
      }
    }
  },
  "io.k8s.api.core.v1.ContainerStateRunning": {
    "description": "ContainerStateRunning is a running state of a container.",
    "type": "object",
    "properties": {
      "startedAt": {
        "description": "Time at which the container was last (re-)started",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      }
    }
  },
  "io.k8s.api.core.v1.ContainerStateTerminated": {
    "description": "ContainerStateTerminated is a terminated state of a container.",
    "type": "object",
    "required": [
      "exitCode"
    ],
    "properties": {
      "containerID": {
        "description": "Container's ID in the format 'docker://<container_id>'",
        "type": "string"
      },
      "exitCode": {
        "description": "Exit status from the last termination of the container",
        "type": "integer",
        "format": "int32"
      },
      "finishedAt": {
        "description": "Time at which the container last terminated",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      },
      "message": {
        "description": "Message regarding the last termination of the container",
        "type": "string"
      },
      "reason": {
        "description": "(brief) reason from the last termination of the container",
        "type": "string"
      },
      "signal": {
        "description": "Signal from the last termination of the container",
        "type": "integer",
        "format": "int32"
      },
      "startedAt": {
        "description": "Time at which previous execution of the container started",
        "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Time"
      }
    }
  },
  "io.k8s.api.core.v1.ContainerStateWaiting": {
    "description": "ContainerStateWaiting is a waiting state of a container.",
    "type": "object",
    "properties": {
      "message": {
        "description": "Message regarding why the container is not yet running.",
        "type": "string"
      },
      "reason": {
        "description": "(brief) reason the container is not yet running.",
        "type": "string"
      }
    }
  },
  "io.k8s.api.core.v1.ObjectReference": {
    "description": "ObjectReference contains enough information to let you inspect or modify the referred object.",
    "type": "object",
    "properties": {
      "apiVersion": {
        "description": "API version of the referent.",
        "type": "string"
      },
      "fieldPath": {
        "description": "If referring to a piece of an object instead of an entire object, this string should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2]. For example, if the object reference is to a container within a pod, this would take on a value like: \"spec.containers{name}\" (where \"name\" refers to the name of the container that triggered the event) or if no container name is specified \"spec.containers[2]\" (container with index 2 in this pod). This syntax is chosen only to have some well-defined way of referencing a part of an object.",
        "type": "string"
      },
      "kind": {
        "description": "Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": "string"
      },
      "name": {
        "description": "Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
        "type": "string"
      },
      "namespace": {
        "description": "Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/",
        "type": "string"
      },
      "resourceVersion": {
        "description": "Specific resourceVersion to which this reference is made, if any. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#concurrency-control-and-consistency",
        "type": "string"
      },
      "uid": {
        "description": "UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids",
        "type": "string"
      }
    }
  },
  "io.k8s.api.core.v1.Toleration": {
	"description": "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
	"properties": {
      "effect": {
	    "description": "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
	    "type": "string"
	  },
	  "key": {
	    "description": "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
	    "type": "string"
	  },
	  "operator": {
        "description": "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
        "type": "string"
	  },
	  "tolerationSeconds": {
	    "description": "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
	    "format": "int64",
	    "type": "integer"
	  },
	  "value": {
	    "description": "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
	    "type": "string"
	  }
	},
	"type": "object"
  },
  "io.k8s.api.core.v1.Affinity": {
    "description": "Affinity is a group of affinity scheduling rules.",
    "properties": {
      "nodeAffinity": {
        "$ref": "#/definitions/io.k8s.api.core.v1.NodeAffinity",
        "description": "Describes node affinity scheduling rules for the pod."
      },
      "podAffinity": {
        "$ref": "#/definitions/io.k8s.api.core.v1.PodAffinity",
        "description": "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s))."
      },
      "podAntiAffinity": {
        "$ref": "#/definitions/io.k8s.api.core.v1.PodAntiAffinity",
        "description": "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s))."
      }
    },
    "type": "object"
  },
  "io.k8s.api.core.v1.NodeAffinity": {
    "description": "Node affinity is a group of node affinity scheduling rules.",
    "properties": {
      "preferredDuringSchedulingIgnoredDuringExecution": {
        "description": "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.PreferredSchedulingTerm"
        },
        "type": "array"
      },
      "requiredDuringSchedulingIgnoredDuringExecution": {
        "$ref": "#/definitions/io.k8s.api.core.v1.NodeSelector",
        "description": "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node."
      }
    },
    "type": "object"
  },
  "io.k8s.api.core.v1.PodAntiAffinity": {
    "description": "Pod anti affinity is a group of inter pod anti affinity scheduling rules.",
    "properties": {
      "preferredDuringSchedulingIgnoredDuringExecution": {
        "description": "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.WeightedPodAffinityTerm"
        },
        "type": "array"
      },
      "requiredDuringSchedulingIgnoredDuringExecution": {
        "description": "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.PodAffinityTerm"
        },
        "type": "array"
      }
    },
    "type": "object"
  },
  "io.k8s.api.core.v1.PodAffinity": {
    "description": "Pod affinity is a group of inter pod affinity scheduling rules.",
    "properties": {
      "preferredDuringSchedulingIgnoredDuringExecution": {
        "description": "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.WeightedPodAffinityTerm"
        },
        "type": "array"
      },
      "requiredDuringSchedulingIgnoredDuringExecution": {
        "description": "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.PodAffinityTerm"
        },
        "type": "array"
      }
    },
    "type": "object"
  },
  "io.k8s.api.core.v1.PreferredSchedulingTerm": {
    "description": "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
    "properties": {
      "preference": {
        "$ref": "#/definitions/io.k8s.api.core.v1.NodeSelectorTerm",
        "description": "A node selector term, associated with the corresponding weight."
      },
      "weight": {
        "description": "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
        "format": "int32",
        "type": "integer"
      }
    },
    "required": [
      "weight",
      "preference"
    ],
    "type": "object"
  },
  "io.k8s.api.core.v1.NodeSelector": {
    "description": "A node selector represents the union of the results of one or more label queries over a set of nodes; that is, it represents the OR of the selectors represented by the node selector terms.",
    "properties": {
      "nodeSelectorTerms": {
        "description": "Required. A list of node selector terms. The terms are ORed.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.NodeSelectorTerm"
        },
        "type": "array"
      }
    },
    "required": [
      "nodeSelectorTerms"
    ],
    "type": "object",
    "x-kubernetes-map-type": "atomic"
  },
  "io.k8s.api.core.v1.NodeSelectorRequirement": {
    "description": "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
    "properties": {
      "key": {
        "description": "The label key that the selector applies to.",
        "type": "string"
      },
      "operator": {
        "description": "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
        "type": "string"
      },
      "values": {
        "description": "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
        "items": {
          "type": "string"
        },
        "type": "array"
      }
    },
    "required": [
      "key",
      "operator"
    ],
    "type": "object"
  },
  "io.k8s.api.core.v1.NodeSelectorTerm": {
    "description": "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
    "properties": {
      "matchExpressions": {
        "description": "A list of node selector requirements by node's labels.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.NodeSelectorRequirement"
        },
        "type": "array"
      },
      "matchFields": {
        "description": "A list of node selector requirements by node's fields.",
        "items": {
          "$ref": "#/definitions/io.k8s.api.core.v1.NodeSelectorRequirement"
        },
        "type": "array"
      }
    },
    "type": "object",
    "x-kubernetes-map-type": "atomic"
  },
  "io.k8s.api.core.v1.PodAffinityTerm": {
    "description": "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
    "properties": {
	    "labelSelector": {
	      "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelector",
	      "description": "A label query over a set of resources, in this case pods."
	    },
	    "namespaceSelector": {
	      "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelector",
	      "description": "A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means \"this pod's namespace\". An empty selector ({}) matches all namespaces. This field is beta-level and is only honored when PodAffinityNamespaceSelector feature is enabled."
	    },
	    "namespaces": {
	      "description": "namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means \"this pod's namespace\"",
	      "items": {
	        "type": "string"
	      },
	      "type": "array"
	    },
	    "topologyKey": {
	      "description": "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
	      "type": "string"
	    }
    },
    "required": [
        "topologyKey"
    ],
	    "type": "object"
  },
  "io.k8s.api.core.v1.WeightedPodAffinityTerm": {
    "description": "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
    "properties": {
      "podAffinityTerm": {
        "$ref": "#/definitions/io.k8s.api.core.v1.PodAffinityTerm",
        "description": "Required. A pod affinity term, associated with the corresponding weight."
      },
      "weight": {
        "description": "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
        "format": "int32",
        "type": "integer"
      }
    },
    "required": [
      "weight",
      "podAffinityTerm"
    ],
    "type": "object"
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelector": {
    "description": "A label selector is a label query over a set of resources. The result of matchLabels and matchExpressions are ANDed. An empty label selector matches all objects. A null label selector matches no objects.",
    "properties": {
      "matchExpressions": {
        "description": "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
        "items": {
          "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelectorRequirement"
        },
        "type": "array"
      },
      "matchLabels": {
        "additionalProperties": {
          "type": "string"
        },
        "description": "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
        "type": "object"
      }
    },
    "type": "object",
    "x-kubernetes-map-type": "atomic"
  },
  "io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelectorRequirement": {
    "description": "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
    "properties": {
      "key": {
        "description": "key is the label key that the selector applies to.",
        "type": "string",
        "x-kubernetes-patch-merge-key": "key",
        "x-kubernetes-patch-strategy": "merge"
      },
      "operator": {
        "description": "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
        "type": "string"
      },
      "values": {
        "description": "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
        "items": {
          "type": "string"
        },
        "type": "array"
      }
    },
    "required": [
      "key",
      "operator"
    ],
    "type": "object"
  }
}`

const namespaceCRDPathsTemplate = `{
  "/apis/{{.Group}}/{{.Version}}/{{.Plural}}": {
    "parameters": [
      {
        "uniqueItems": true,
        "type": "string",
        "description": "If 'true', then the output is pretty printed.",
        "name": "pretty",
        "in": "query"
      }
    ],
    "get": {
      "operationId": "listAll{{.Plural | Title}}",
      "description": "list or watch {{.Plural}}",
      "tags": [
        "kpack"
      ],
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/json;stream=watch"
      ],
      "schemes": [
        "https"
      ],
      "parameters": [
        {
          "uniqueItems": true,
          "type": "string",
          "description": "The continue option should be set when retrieving more results from the server. Since this value is server defined, clients may only use the continue value from a previous query result with identical query parameters (except for the value of continue) and the server may reject a continue value it does not recognize. If the specified continue value is no longer valid whether due to expiration (generally five to fifteen minutes) or a configuration change on the server, the server will respond with a 410 ResourceExpired error together with a continue token. If the client needs a consistent list, it must restart their list without the continue field. Otherwise, the client may send another list request with the token received with the 410 error, the server will respond with a list starting from the next key, but from the latest snapshot, which is inconsistent from the previous list results - objects that are created, modified, or deleted after the first list request will be included in the response, as long as their keys are after the \"next key\".\n\nThis field is not supported when watch is true. Clients may start a watch from the last resourceVersion value returned by the server and not miss any modifications.",
          "name": "continue",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their fields. Defaults to everything.",
          "name": "fieldSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their labels. Defaults to everything.",
          "name": "labelSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "limit is a maximum number of responses to return for a list call. If more items exist, the server will set the 'continue' field on the list metadata to a value that can be used with the same initial query to retrieve the next set of results. Setting a limit may return fewer than the requested amount of items (up to zero items) in the event all requested objects are filtered out and clients should only use the presence of the continue field to determine whether more results are available. Servers may choose not to support the limit argument and will return all of the available results. If limit is specified and the continue field is empty, clients may assume that no more results are available. This field is not supported if watch is true.\n\nThe server guarantees that the objects returned when using continue will be identical to issuing a single list call without a limit - that is, no objects created, modified, or deleted after the first request is issued will be included in any subsequent continued requests. This is sometimes referred to as a consistent snapshot, and ensures that a client that is using limit to receive smaller chunks of a very large result can ensure they see all possible objects. If objects are updated during a chunked list the version of the object that was present at the time the first list result was calculated is returned.",
          "name": "limit",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "When specified with a watch call, shows changes that occur after that particular version of a resource. Defaults to changes from the beginning of history. When specified for list: - if unset, then the result is returned from remote storage based on quorum-read flag; - if it's 0, then we simply return what we currently have in cache, no guarantee; - if set to non zero, then the result is at least as fresh as given rv.",
          "name": "resourceVersion",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "Timeout for the list/watch call. This limits the duration of the call, regardless of any activity or inactivity.",
          "name": "timeoutSeconds",
          "in": "query"
        },
        {
          "name": "watch",
          "uniqueItems": true,
          "type": "boolean",
          "description": "Watch for changes to the described resources and return them as a stream of add, update, and remove notifications.",
          "in": "query"
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.ListRef}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  },
  "/apis/{{.Group}}/{{.Version}}/namespaces/{namespace}/{{.Plural}}": {
    "parameters": [
      {
        "uniqueItems": true,
        "type": "string",
        "description": "If 'true', then the output is pretty printed.",
        "name": "pretty",
        "in": "query"
      },
      {
        "name": "namespace",
        "in": "path",
        "required": true,
        "description": "The custom resource's namespace",
        "type": "string"
      }
    ],
    "get": {
      "operationId": "listNamespaced{{.Plural | Title}}",
      "description": "list or watch namespace scoped {{.Plural}}",
      "tags": [
        "kpack"
      ],
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/json;stream=watch"
      ],
      "schemes": [
        "https"
      ],
      "parameters": [
        {
          "uniqueItems": true,
          "type": "string",
          "description": "The continue option should be set when retrieving more results from the server. Since this value is server defined, clients may only use the continue value from a previous query result with identical query parameters (except for the value of continue) and the server may reject a continue value it does not recognize. If the specified continue value is no longer valid whether due to expiration (generally five to fifteen minutes) or a configuration change on the server, the server will respond with a 410 ResourceExpired error together with a continue token. If the client needs a consistent list, it must restart their list without the continue field. Otherwise, the client may send another list request with the token received with the 410 error, the server will respond with a list starting from the next key, but from the latest snapshot, which is inconsistent from the previous list results - objects that are created, modified, or deleted after the first list request will be included in the response, as long as their keys are after the \"next key\".\n\nThis field is not supported when watch is true. Clients may start a watch from the last resourceVersion value returned by the server and not miss any modifications.",
          "name": "continue",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their fields. Defaults to everything.",
          "name": "fieldSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their labels. Defaults to everything.",
          "name": "labelSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "limit is a maximum number of responses to return for a list call. If more items exist, the server will set the 'continue' field on the list metadata to a value that can be used with the same initial query to retrieve the next set of results. Setting a limit may return fewer than the requested amount of items (up to zero items) in the event all requested objects are filtered out and clients should only use the presence of the continue field to determine whether more results are available. Servers may choose not to support the limit argument and will return all of the available results. If limit is specified and the continue field is empty, clients may assume that no more results are available. This field is not supported if watch is true.\n\nThe server guarantees that the objects returned when using continue will be identical to issuing a single list call without a limit - that is, no objects created, modified, or deleted after the first request is issued will be included in any subsequent continued requests. This is sometimes referred to as a consistent snapshot, and ensures that a client that is using limit to receive smaller chunks of a very large result can ensure they see all possible objects. If objects are updated during a chunked list the version of the object that was present at the time the first list result was calculated is returned.",
          "name": "limit",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "When specified with a watch call, shows changes that occur after that particular version of a resource. Defaults to changes from the beginning of history. When specified for list: - if unset, then the result is returned from remote storage based on quorum-read flag; - if it's 0, then we simply return what we currently have in cache, no guarantee; - if set to non zero, then the result is at least as fresh as given rv.",
          "name": "resourceVersion",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "Timeout for the list/watch call. This limits the duration of the call, regardless of any activity or inactivity.",
          "name": "timeoutSeconds",
          "in": "query"
        },
        {
          "name": "watch",
          "uniqueItems": true,
          "type": "boolean",
          "description": "Watch for changes to the described resources and return them as a stream of add, update, and remove notifications.",
          "in": "query"
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.ListRef}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "post": {
      "operationId": "create{{.Kind}}",
      "description": "Creates a namespace scoped {{.Kind}}",
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to create.",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "201": {
          "description": "Created",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  },
  "/apis/{{.Group}}/{{.Version}}/namespaces/{namespace}/{{.Plural}}/{name}": {
    "parameters": [
      {
        "name": "namespace",
        "in": "path",
        "required": true,
        "description": "The custom resource's namespace",
        "type": "string"
      },
      {
        "name": "name",
        "in": "path",
        "required": true,
        "description": "the custom object's name",
        "type": "string"
      }
    ],
    "get": {
      "operationId": "get{{.Kind}}",
      "description": "Returns a namespace scoped custom object",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "responses": {
        "200": {
          "description": "A single Resource",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "delete": {
      "operationId": "delete{{.Kind}}",
      "description": "Deletes the specified namespace scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions"
          }
        },
        {
          "name": "gracePeriodSeconds",
          "uniqueItems": true,
          "type": "integer",
          "description": "The duration in seconds before the object should be deleted. Value must be non-negative integer. The value zero indicates delete immediately. If this value is nil, the default grace period for the specified type will be used. Defaults to a per object value if not specified. zero means delete immediately.",
          "in": "query"
        },
        {
          "name": "orphanDependents",
          "uniqueItems": true,
          "type": "boolean",
          "description": "Deprecated: please use the PropagationPolicy, this field will be deprecated in 1.7. Should the dependent objects be orphaned. If true/false, the \"orphan\" finalizer will be added to/removed from the object's finalizers list. Either this field or PropagationPolicy may be set, but not both.",
          "in": "query"
        },
        {
          "name": "propagationPolicy",
          "uniqueItems": true,
          "type": "string",
          "description": "Whether and how garbage collection will be performed. Either this field or OrphanDependents may be set, but not both. The default policy is decided by the existing finalizer set in the metadata.finalizers and the resource-specific default policy.",
          "in": "query"
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Status"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "patch": {
      "operationId": "patch{{.Kind}}",
      "description": "patch the specified namespace scoped {{.Kind}}",
      "consumes": [
        "application/json-patch+json",
        "application/merge-patch+json"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to patch.",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Patch"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "put": {
      "operationId": "replace{{.Kind}}",
      "description": "replace the specified namespace scoped custom object",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to replace.",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  },
  "/apis/{{.Group}}/{{.Version}}/namespaces/{namespace}/{{.Plural}}/{name}/status": {
    "parameters": [
      {
        "name": "namespace",
        "in": "path",
        "required": true,
        "description": "The custom resource's namespace",
        "type": "string"
      },
      {
        "name": "name",
        "in": "path",
        "required": true,
        "description": "the custom object's name",
        "type": "string"
      }
    ],
    "get": {
      "description": "read status of the specified namespace scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "get{{.Kind}}Status",
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "put": {
      "description": "replace status of the specified namespace scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "replace{{.Kind}}Status",
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "201": {
          "description": "Created",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "patch": {
      "description": "partially update status of the specified namespace scoped {{.Kind}}",
      "consumes": [
        "application/json-patch+json",
        "application/merge-patch+json"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "patch{{.Kind}}Status",
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Patch"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  }
}
`

const clusterCRDPathsTemplate = `{
  "/apis/{{.Group}}/{{.Version}}/{{.Plural}}": {
    "parameters": [
      {
        "uniqueItems": true,
        "type": "string",
        "description": "If 'true', then the output is pretty printed.",
        "name": "pretty",
        "in": "query"
      }
    ],
    "get": {
      "operationId": "listAll{{.Plural | Title}}",
      "description": "list or watch cluster scoped {{.Plural}}",
      "tags": [
        "kpack"
      ],
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/json;stream=watch"
      ],
      "schemes": [
        "https"
      ],
      "parameters": [
        {
          "uniqueItems": true,
          "type": "string",
          "description": "The continue option should be set when retrieving more results from the server. Since this value is server defined, clients may only use the continue value from a previous query result with identical query parameters (except for the value of continue) and the server may reject a continue value it does not recognize. If the specified continue value is no longer valid whether due to expiration (generally five to fifteen minutes) or a configuration change on the server, the server will respond with a 410 ResourceExpired error together with a continue token. If the client needs a consistent list, it must restart their list without the continue field. Otherwise, the client may send another list request with the token received with the 410 error, the server will respond with a list starting from the next key, but from the latest snapshot, which is inconsistent from the previous list results - objects that are created, modified, or deleted after the first list request will be included in the response, as long as their keys are after the \"next key\".\n\nThis field is not supported when watch is true. Clients may start a watch from the last resourceVersion value returned by the server and not miss any modifications.",
          "name": "continue",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their fields. Defaults to everything.",
          "name": "fieldSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "A selector to restrict the list of returned objects by their labels. Defaults to everything.",
          "name": "labelSelector",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "limit is a maximum number of responses to return for a list call. If more items exist, the server will set the 'continue' field on the list metadata to a value that can be used with the same initial query to retrieve the next set of results. Setting a limit may return fewer than the requested amount of items (up to zero items) in the event all requested objects are filtered out and clients should only use the presence of the continue field to determine whether more results are available. Servers may choose not to support the limit argument and will return all of the available results. If limit is specified and the continue field is empty, clients may assume that no more results are available. This field is not supported if watch is true.\n\nThe server guarantees that the objects returned when using continue will be identical to issuing a single list call without a limit - that is, no objects created, modified, or deleted after the first request is issued will be included in any subsequent continued requests. This is sometimes referred to as a consistent snapshot, and ensures that a client that is using limit to receive smaller chunks of a very large result can ensure they see all possible objects. If objects are updated during a chunked list the version of the object that was present at the time the first list result was calculated is returned.",
          "name": "limit",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "string",
          "description": "When specified with a watch call, shows changes that occur after that particular version of a resource. Defaults to changes from the beginning of history. When specified for list: - if unset, then the result is returned from remote storage based on quorum-read flag; - if it's 0, then we simply return what we currently have in cache, no guarantee; - if set to non zero, then the result is at least as fresh as given rv.",
          "name": "resourceVersion",
          "in": "query"
        },
        {
          "uniqueItems": true,
          "type": "integer",
          "description": "Timeout for the list/watch call. This limits the duration of the call, regardless of any activity or inactivity.",
          "name": "timeoutSeconds",
          "in": "query"
        },
        {
          "name": "watch",
          "uniqueItems": true,
          "type": "boolean",
          "description": "Watch for changes to the described resources and return them as a stream of add, update, and remove notifications.",
          "in": "query"
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.ListRef}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "post": {
      "operationId": "create{{.Kind}}",
      "description": "Creates a cluster scoped {{.Kind}}",
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to create.",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "201": {
          "description": "Created",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  },
  "/apis/{{.Group}}/{{.Version}}/{{.Plural}}/{name}": {
    "parameters": [
      {
        "name": "name",
        "in": "path",
        "required": true,
        "description": "the custom object's name",
        "type": "string"
      }
    ],
    "get": {
      "operationId": "get{{.Kind}}",
      "description": "Returns a cluster scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "delete": {
      "operationId": "delete{{.Kind}}",
      "description": "Deletes the specified cluster scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions"
          }
        },
        {
          "name": "gracePeriodSeconds",
          "uniqueItems": true,
          "type": "integer",
          "description": "The duration in seconds before the object should be deleted. Value must be non-negative integer. The value zero indicates delete immediately. If this value is nil, the default grace period for the specified type will be used. Defaults to a per object value if not specified. zero means delete immediately.",
          "in": "query"
        },
        {
          "name": "orphanDependents",
          "uniqueItems": true,
          "type": "boolean",
          "description": "Deprecated: please use the PropagationPolicy, this field will be deprecated in 1.7. Should the dependent objects be orphaned. If true/false, the \"orphan\" finalizer will be added to/removed from the object's finalizers list. Either this field or PropagationPolicy may be set, but not both.",
          "in": "query"
        },
        {
          "name": "propagationPolicy",
          "uniqueItems": true,
          "type": "string",
          "description": "Whether and how garbage collection will be performed. Either this field or OrphanDependents may be set, but not both. The default policy is decided by the existing finalizer set in the metadata.finalizers and the resource-specific default policy.",
          "in": "query"
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Status"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "patch": {
      "operationId": "patch{{.Kind}}",
      "description": "patch the specified cluster scoped {{.Kind}}",
      "consumes": [
        "application/json-patch+json",
        "application/merge-patch+json"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to patch.",
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Patch"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "put": {
      "operationId": "replace{{.Kind}}",
      "description": "replace the specified cluster scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "description": "The JSON schema of the Resource to replace.",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  },
  "/apis/{{.Group}}/{{.Version}}/{{.Plural}}/{name}/status": {
    "parameters": [
      {
        "name": "name",
        "in": "path",
        "required": true,
        "description": "the custom object's name",
        "type": "string"
      }
    ],
    "get": {
      "description": "read status of the specified cluster scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "get{{.Kind}}Status",
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "put": {
      "description": "replace status of the specified cluster scoped {{.Kind}}",
      "consumes": [
        "*/*"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "replace{{.Kind}}Status",
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "schema": {
            "$ref": "{{.Ref}}"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "201": {
          "description": "Created",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    },
    "patch": {
      "description": "partially update status of the specified cluster scoped {{.Kind}}",
      "consumes": [
        "application/json-patch+json",
        "application/merge-patch+json"
      ],
      "produces": [
        "application/json",
        "application/yaml",
        "application/vnd.kubernetes.protobuf"
      ],
      "schemes": [
        "https"
      ],
      "tags": [
        "kpack"
      ],
      "operationId": "patch{{.Kind}}Status",
      "parameters": [
        {
          "name": "body",
          "in": "body",
          "required": true,
          "schema": {
            "$ref": "#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.Patch"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "OK",
          "schema": {
            "$ref": "{{.Ref}}"
          }
        },
        "401": {
          "description": "Unauthorized"
        }
      }
    }
  }
}
`
