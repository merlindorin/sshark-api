package postgres

type FieldMapping struct {
	Column  string
	IsArray bool
}

//nolint:gochecknoglobals // immutable field mappings
var SSHFieldMapping = map[string]FieldMapping{
	"id":              {Column: "pk.id"},
	"fingerprint":     {Column: "pk.fingerprint"},
	"algorithm":       {Column: "sm.algorithm"},
	"comment":         {Column: "sm.comment"},
	"key_bits":        {Column: "sm.key_bits"},
	"source.username": {Column: "s.username"},
	"source.provider": {Column: "s.provider"},
	"source.user_id":  {Column: "s.user_id"},
	"source.uri":      {Column: "s.uri"},
}

//nolint:gochecknoglobals // immutable field mappings
var GPGFieldMapping = map[string]FieldMapping{
	"id":              {Column: "pk.id"},
	"fingerprint":     {Column: "pk.fingerprint"},
	"algorithm":       {Column: "gm.algorithm"},
	"key_bits":        {Column: "gm.key_bits"},
	"user_ids":        {Column: "gm.user_ids", IsArray: true},
	"capabilities":    {Column: "gm.capabilities", IsArray: true},
	"source.username": {Column: "s.username"},
	"source.provider": {Column: "s.provider"},
	"source.user_id":  {Column: "s.user_id"},
	"source.uri":      {Column: "s.uri"},
}
