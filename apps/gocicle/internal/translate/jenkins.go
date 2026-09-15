package translate

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
)

type lexeme struct {
	value        string
	quoted       bool
	line, column int
}
type declarative struct {
	tokens []lexeme
	at     int
	file   string
	result *Result
}

func lex(input string) ([]lexeme, error) {
	tokens := []lexeme{}
	line, column := 1, 1
	for i := 0; i < len(input); {
		c := input[i]
		if c == '\n' {
			line++
			column = 1
			i++
			continue
		}
		if unicode.IsSpace(rune(c)) {
			i++
			column++
			continue
		}
		if strings.HasPrefix(input[i:], "//") {
			for i < len(input) && input[i] != '\n' {
				i++
				column++
			}
			continue
		}
		token := lexeme{line: line, column: column}
		switch {
		case c == '\'' || c == '"':
			token.quoted = true
			delimiter := string(c)
			if strings.HasPrefix(input[i:], strings.Repeat(delimiter, 3)) {
				delimiter = strings.Repeat(delimiter, 3)
			}
			i += len(delimiter)
			column += len(delimiter)
			var value strings.Builder
			for i < len(input) && !strings.HasPrefix(input[i:], delimiter) {
				if input[i] == '\\' && i+1 < len(input) {
					next := input[i+1]
					if next == c || next == '\\' {
						value.WriteByte(next)
						i += 2
						column += 2
						continue
					}
				}
				if input[i] == '\n' {
					line++
					column = 1
				} else {
					column++
				}
				value.WriteByte(input[i])
				i++
			}
			if i == len(input) {
				return tokens, fmt.Errorf("%d:%d: unterminated string", token.line, token.column)
			}
			i += len(delimiter)
			column += len(delimiter)
			token.value = value.String()
			if c == '"' && strings.Contains(token.value, "$") {
				return tokens, fmt.Errorf("%d:%d: Groovy interpolation is unsupported", token.line, token.column)
			}
		case strings.ContainsRune("{}()=;", rune(c)):
			token.value = string(c)
			i++
			column++
		default:
			start := i
			for i < len(input) && (unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i])) || input[i] == '_' || input[i] == '-') {
				i++
				column++
			}
			if start == i {
				return tokens, fmt.Errorf("%d:%d: unsupported Groovy token", line, column)
			}
			token.value = input[start:i]
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}
func (p *declarative) peek() string {
	if p.at >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.at].value
}
func (p *declarative) take(value string) error {
	if p.at >= len(p.tokens) || p.tokens[p.at].quoted || p.peek() != value {
		return p.fail("expected " + value)
	}
	p.at++
	return nil
}
func (p *declarative) fail(message string) error {
	line, column := 1, 1
	if p.at < len(p.tokens) {
		line, column = p.tokens[p.at].line, p.tokens[p.at].column
	}
	return fmt.Errorf("%d:%d: %s", line, column, message)
}
func (p *declarative) literal() (string, error) {
	parens := p.peek() == "("
	if parens {
		p.at++
	}
	if p.at >= len(p.tokens) || !p.tokens[p.at].quoted {
		return "", p.fail("expected a literal string")
	}
	value := p.peek()
	p.at++
	if parens {
		if err := p.take(")"); err != nil {
			return "", err
		}
	}
	if p.peek() == ";" {
		p.at++
	}
	return value, nil
}
func (r *Result) jenkins(file, input string, o Options) {
	tokens, err := lex(input)
	spec := jobs.Spec{Timezone: "UTC", Container: jobs.Container{Image: o.Image}, Command: jobs.Command{Path: "/bin/sh", WorkingDirectory: "/workspace"}, Environment: map[string]string{}}
	p := declarative{tokens: tokens, file: file, result: r}
	commands := []string{}
	if err == nil {
		err = func() error {
			for _, s := range []string{"pipeline", "{"} {
				if err := p.take(s); err != nil {
					return err
				}
			}
			seen := map[string]bool{}
			for p.peek() != "}" && p.peek() != "" {
				field := p.peek()
				if seen[field] {
					return p.fail("duplicate pipeline section")
				}
				seen[field] = true
				p.at++
				if err := p.take("{"); err != nil {
					return err
				}
				switch field {
				case "agent":
					if err := p.take("docker"); err != nil {
						return err
					}
					if p.peek() == "{" {
						p.at++
						if err := p.take("image"); err != nil {
							return err
						}
						image, err := p.literal()
						if err != nil {
							return err
						}
						spec.Container.Image = image
						if err := p.take("}"); err != nil {
							return err
						}
					} else {
						image, err := p.literal()
						if err != nil {
							return err
						}
						spec.Container.Image = image
					}
				case "triggers":
					if err := p.take("cron"); err != nil {
						return err
					}
					schedule, err := p.literal()
					if err != nil {
						return err
					}
					if strings.Contains(schedule, "H") {
						return p.fail("Jenkins hashed cron is unsupported")
					}
					spec.Schedule = schedule
				case "environment":
					for p.peek() != "}" && p.peek() != "" {
						name := p.peek()
						p.at++
						if err := p.take("="); err != nil {
							return err
						}
						value, err := p.literal()
						if err != nil {
							return err
						}
						if _, ok := spec.Environment[name]; ok {
							return p.fail("duplicate environment variable")
						}
						spec.Environment[name] = value
					}
				case "stages":
					for p.peek() != "}" && p.peek() != "" {
						if err := p.take("stage"); err != nil {
							return err
						}
						if _, err := p.literal(); err != nil {
							return err
						}
						for _, s := range []string{"{", "steps", "{"} {
							if err := p.take(s); err != nil {
								return err
							}
						}
						for p.peek() != "}" && p.peek() != "" {
							if err := p.take("sh"); err != nil {
								return err
							}
							command, err := p.literal()
							if err != nil {
								return err
							}
							commands = append(commands, "/bin/sh -xe -c "+shellQuote(command))
						}
						for _, s := range []string{"}", "}"} {
							if err := p.take(s); err != nil {
								return err
							}
						}
					}
				default:
					return p.fail("unsupported pipeline section: " + field)
				}
				if err := p.take("}"); err != nil {
					return err
				}
			}
			if err := p.take("}"); err != nil {
				return err
			}
			if p.at != len(p.tokens) {
				return p.fail("unsupported code after pipeline")
			}
			if !seen["agent"] || !seen["stages"] || len(commands) == 0 {
				return p.fail("pipeline needs one Docker agent and sequential shell stages")
			}
			if labels, ok := o.Labels["jenkins"]; ok {
				spec.Runner.Labels = labels
			} else {
				return p.fail("map the jenkins runner label explicitly")
			}
			return nil
		}()
	}
	r.script("pipeline", &spec, commands)
	r.Document.Jobs["pipeline"] = spec
	if err != nil {
		line, column := 1, 1
		message := err.Error()
		_, _ = fmt.Sscanf(message, "%d:%d:", &line, &column)
		r.Diagnostics = append(r.Diagnostics, Diagnostic{File: file, Line: line, Column: column, Message: message})
	}
}
