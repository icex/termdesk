package launcher

// TokenizeCommand splits a command string into tokens, respecting
// single and double quotes (shell-like). Unmatched trailing quotes
// treat the rest of the string as a single token.
func TokenizeCommand(input string) []string {
	var tokens []string
	var cur []byte
	i := 0
	for i < len(input) {
		ch := input[i]
		switch {
		case ch == ' ' || ch == '\t':
			if len(cur) > 0 {
				tokens = append(tokens, string(cur))
				cur = cur[:0]
			}
			i++
		case ch == '"' || ch == '\'':
			i++ // skip opening quote
			for i < len(input) && input[i] != ch {
				if input[i] == '\\' && i+1 < len(input) {
					i++ // skip escape
				}
				cur = append(cur, input[i])
				i++
			}
			if i < len(input) {
				i++ // skip closing quote
			}
		default:
			cur = append(cur, ch)
			i++
		}
	}
	if len(cur) > 0 {
		tokens = append(tokens, string(cur))
	}
	return tokens
}
