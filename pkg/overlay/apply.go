package overlay

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type pathToken interface {
	apply([]nodeRef) ([]nodeRef, error)
}

type nodeRef struct {
	value  any
	set    func(any)
	remove func()
}

type fieldToken struct{ name string }
type indexToken struct{ index int }
type filterToken struct {
	field string
	value string
}

func Apply(base map[string]any, doc Document) (map[string]any, error) {
	cloned, err := deepCopy(base)
	if err != nil {
		return nil, err
	}

	for _, action := range doc.Actions {
		tokens, err := parseJSONPath(action.Target)
		if err != nil {
			return nil, err
		}
		matches, err := findMatches(nodeRef{value: cloned}, tokens)
		if err != nil {
			return nil, err
		}

		if action.Update != nil {
			for _, match := range matches {
				target, ok := match.value.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("update target %q is not an object", action.Target)
				}
				for key, value := range action.Update {
					target[key] = value
				}
			}
		}

		if action.Copy != nil {
			copyTokens, err := parseJSONPath(action.Copy.To)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("copy source %q matched no nodes", action.Target)
			}
			if err := setPath(cloned, copyTokens, matches[0].value); err != nil {
				return nil, err
			}
		}

		if action.Remove {
			for i := len(matches) - 1; i >= 0; i-- {
				if matches[i].remove == nil {
					return nil, fmt.Errorf("remove target %q cannot be removed", action.Target)
				}
				matches[i].remove()
			}
		}
	}

	return cloned, nil
}

func parseJSONPath(expression string) ([]pathToken, error) {
	if expression == "" || expression[0] != '$' {
		return nil, fmt.Errorf("invalid jsonpath %q", expression)
	}

	var tokens []pathToken
	for i := 1; i < len(expression); {
		switch expression[i] {
		case '.':
			i++
			start := i
			for i < len(expression) && isIdentifier(expression[i]) {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("invalid jsonpath %q", expression)
			}
			tokens = append(tokens, fieldToken{name: expression[start:i]})
		case '[':
			if strings.HasPrefix(expression[i:], "['") || strings.HasPrefix(expression[i:], "[\"") {
				quote := expression[i+1]
				i += 2
				start := i
				for i < len(expression) && expression[i] != quote {
					i++
				}
				if i >= len(expression) || i+1 >= len(expression) || expression[i+1] != ']' {
					return nil, fmt.Errorf("invalid jsonpath %q", expression)
				}
				tokens = append(tokens, fieldToken{name: expression[start:i]})
				i += 2
				continue
			}
			if strings.HasPrefix(expression[i:], "[?(") {
				end := strings.Index(expression[i:], ")]")
				if end == -1 {
					return nil, fmt.Errorf("invalid jsonpath %q", expression)
				}
				raw := expression[i : i+end+2]
				token, err := parseFilter(raw)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, token)
				i += end + 2
				continue
			}

			end := strings.IndexByte(expression[i:], ']')
			if end == -1 {
				return nil, fmt.Errorf("invalid jsonpath %q", expression)
			}
			indexValue := expression[i+1 : i+end]
			index, err := strconv.Atoi(indexValue)
			if err != nil {
				return nil, fmt.Errorf("invalid jsonpath index in %q", expression)
			}
			tokens = append(tokens, indexToken{index: index})
			i += end + 1
		default:
			return nil, fmt.Errorf("invalid jsonpath %q", expression)
		}
	}

	return tokens, nil
}

var filterPattern = regexp.MustCompile(`^\[\?\(@\.([A-Za-z0-9_-]+)\s*==\s*'([^']*)'\)\]$`)

func parseFilter(raw string) (pathToken, error) {
	matches := filterPattern.FindStringSubmatch(raw)
	if len(matches) != 3 {
		return nil, fmt.Errorf("unsupported jsonpath filter %q", raw)
	}
	return filterToken{field: matches[1], value: matches[2]}, nil
}

func (token fieldToken) apply(nodes []nodeRef) ([]nodeRef, error) {
	var results []nodeRef
	for _, node := range nodes {
		object, ok := node.value.(map[string]any)
		if !ok {
			continue
		}
		value, ok := object[token.name]
		if !ok {
			continue
		}
		currentObject := object
		key := token.name
		results = append(results, nodeRef{
			value: value,
			set: func(v any) {
				currentObject[key] = v
			},
			remove: func() {
				delete(currentObject, key)
			},
		})
	}
	return results, nil
}

func (token indexToken) apply(nodes []nodeRef) ([]nodeRef, error) {
	var results []nodeRef
	for _, node := range nodes {
		items, ok := node.value.([]any)
		if !ok || token.index < 0 || token.index >= len(items) {
			continue
		}
		index := token.index
		snapshot := items
		parentSet := node.set
		results = append(results, nodeRef{
			value: snapshot[index],
			set: func(v any) {
				snapshot[index] = v
				if parentSet != nil {
					parentSet(snapshot)
				}
			},
			remove: func() {
				updated := append(append([]any{}, snapshot[:index]...), snapshot[index+1:]...)
				if parentSet != nil {
					parentSet(updated)
				}
			},
		})
	}
	return results, nil
}

func (token filterToken) apply(nodes []nodeRef) ([]nodeRef, error) {
	var results []nodeRef
	for _, node := range nodes {
		items, ok := node.value.([]any)
		if !ok {
			continue
		}
		for idx, item := range items {
			object, ok := item.(map[string]any)
			if !ok || fmt.Sprint(object[token.field]) != token.value {
				continue
			}
			index := idx
			snapshot := items
			parentSet := node.set
			results = append(results, nodeRef{
				value: object,
				set: func(v any) {
					snapshot[index] = v
					if parentSet != nil {
						parentSet(snapshot)
					}
				},
				remove: func() {
					updated := append(append([]any{}, snapshot[:index]...), snapshot[index+1:]...)
					if parentSet != nil {
						parentSet(updated)
					}
				},
			})
		}
	}
	return results, nil
}

func findMatches(root nodeRef, tokens []pathToken) ([]nodeRef, error) {
	current := []nodeRef{root}
	var err error
	for _, token := range tokens {
		current, err = token.apply(current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func setPath(root map[string]any, tokens []pathToken, value any) error {
	current := any(root)
	for i, token := range tokens {
		isLast := i == len(tokens)-1
		field, ok := token.(fieldToken)
		if !ok {
			return fmt.Errorf("copy destination only supports object fields")
		}

		object, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("copy destination traverses non-object")
		}

		if isLast {
			cloned, err := deepCopyValue(value)
			if err != nil {
				return err
			}
			object[field.name] = cloned
			return nil
		}

		next, ok := object[field.name]
		if !ok {
			child := map[string]any{}
			object[field.name] = child
			next = child
		}
		current = next
	}
	return nil
}

func deepCopy(source map[string]any) (map[string]any, error) {
	value, err := deepCopyValue(source)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object clone")
	}
	return object, nil
}

func deepCopyValue(source any) (any, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var cloned any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func isIdentifier(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-'
}
