package query

type Query struct {
	Expression *OrExpr `parser:"@@"`
}

type OrExpr struct {
	Left  *AndExpr   `parser:"@@"`
	Right []*AndExpr `parser:"( '|' @@ )*"`
}

type AndExpr struct {
	Left  *Unary   `parser:"@@"`
	Right []*Unary `parser:"( '&' @@ )*"`
}

type Unary struct {
	SubExpr *OrExpr `parser:"  '(' @@ ')'"`
	Field   *Field  `parser:"| @@"`
}

type Field struct {
	Name  string `parser:"'@' @Ident ':'"`
	Value *Value `parser:"'{' @@ '}'"`
}

type Value struct {
	Raw string `parser:"@(Ident | Number | Wildcard | ValueChar | Escaped)+"`
}

func (v *Value) String() string {
	return unescape(v.Raw)
}

func (v *Value) HasWildcard() bool {
	escaped := false
	for _, c := range v.Raw {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '*' {
			return true
		}
	}
	return false
}

func unescape(s string) string {
	result := make([]byte, 0, len(s))
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			result = append(result, s[i])
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
