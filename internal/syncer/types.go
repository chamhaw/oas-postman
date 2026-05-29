package syncer

type Options struct {
	SpecPath       string
	OutputDir      string
	CollectionName string
	BaseURL        string
	FolderStrategy string
	OrphanPolicy   string
	DryRun         bool
	Verbose        bool
}

type Result struct {
	OutputDir             string
	OperationCount        int
	PreservedExampleCount int
	GeneratedExampleCount int
	DeprecatedCount       int
}

type Document struct {
	Title       string
	Description string
	Version     string
	BaseURL     string
	Tags        []Tag
	Operations  []Operation
	Root        map[string]any
}

type Tag struct {
	Name        string
	Description string
}

type Operation struct {
	ID          string
	Method      string
	Path        string
	Tag         string
	Summary     string
	Description string
	Parameters  []Parameter
	RequestBody *Payload
	Responses   []Response
	Security    *Security
	Order       int
}

type Security struct {
	Basic bool
}

type Parameter struct {
	Name        string
	In          string
	Description string
	Required    bool
	Schema      map[string]any
	Example     any
}

type Payload struct {
	ContentType string
	Schema      map[string]any
	Example     any
}

type Response struct {
	Status      string
	Description string
	ContentType string
	Schema      map[string]any
	Example     any
}

type CollectionDefinition struct {
	Kind        string            `yaml:"$kind"`
	Name        string            `yaml:"name,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Order       int               `yaml:"order,omitempty"`
}

type KeyValue struct {
	Key         string `yaml:"key"`
	Value       string `yaml:"value,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type RequestFile struct {
	Kind          string            `yaml:"$kind"`
	ID            string            `yaml:"id,omitempty"`
	Name          string            `yaml:"name,omitempty"`
	OperationID   string            `yaml:"operationId,omitempty"`
	Description   string            `yaml:"description,omitempty"`
	URL           string            `yaml:"url"`
	Method        string            `yaml:"method"`
	Headers       map[string]string `yaml:"headers,omitempty"`
	QueryParams   []KeyValue        `yaml:"queryParams,omitempty"`
	PathVariables []KeyValue        `yaml:"pathVariables,omitempty"`
	Body          Body              `yaml:"body"`
	Auth          *Auth             `yaml:"auth,omitempty"`
	Examples      string            `yaml:"examples,omitempty"`
	Order         int               `yaml:"order,omitempty"`
	Sync          SyncMeta          `yaml:"x-postman-sync,omitempty"`
}

type Body struct {
	Type    string `yaml:"type"`
	Content string `yaml:"content"`
	Schema  string `yaml:"$type,omitempty"`
}

type Auth struct {
	Type        string            `yaml:"type"`
	Credentials map[string]string `yaml:"credentials"`
}

type SyncMeta struct {
	OperationID string `yaml:"operationId,omitempty"`
	Method      string `yaml:"method,omitempty"`
	Path        string `yaml:"path,omitempty"`
	Generated   bool   `yaml:"generated,omitempty"`
	Orphaned    bool   `yaml:"orphaned,omitempty"`
	StatusCode  string `yaml:"statusCode,omitempty"`
}

type ExampleFile struct {
	Kind     string          `yaml:"$kind"`
	Name     string          `yaml:"name,omitempty"`
	Request  ExampleRequest  `yaml:"request"`
	Response ExampleResponse `yaml:"response"`
	Order    int             `yaml:"order,omitempty"`
	Sync     SyncMeta        `yaml:"x-postman-sync,omitempty"`
}

type ExampleRequest struct {
	URL           string            `yaml:"url"`
	Method        string            `yaml:"method"`
	Headers       map[string]string `yaml:"headers,omitempty"`
	QueryParams   []KeyValue        `yaml:"queryParams,omitempty"`
	PathVariables []KeyValue        `yaml:"pathVariables,omitempty"`
	Body          Body              `yaml:"body,omitempty"`
}

type ExampleResponse struct {
	StatusCode int               `yaml:"statusCode"`
	StatusText string            `yaml:"statusText,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Body       Body              `yaml:"body,omitempty"`
}
