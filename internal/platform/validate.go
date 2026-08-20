package platform

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validate collects validation errors and returns them as a single ErrInvalid.
// It is meant to be used in handlers and services to accumulate field-level
// checks before touching any state.
type Validate struct {
	errs []string
}

// New returns an empty validator.
func NewValidate() *Validate { return &Validate{} }

// Require appends msg when cond is false.
func (v *Validate) Require(cond bool, msg string) *Validate {
	if !cond {
		v.errs = append(v.errs, msg)
	}
	return v
}

// RequireNonEmpty appends msg when s is empty after trimming.
func (v *Validate) RequireNonEmpty(field, s string) *Validate {
	if strings.TrimSpace(s) == "" {
		v.errs = append(v.errs, field+" must not be empty")
	}
	return v
}

// RequirePositive appends msg when x <= 0.
func (v *Validate) RequirePositive(field string, x float64) *Validate {
	if x <= 0 {
		v.errs = append(v.errs, field+" must be positive")
	}
	return v
}

// RequireNonNegative appends msg when x < 0.
func (v *Validate) RequireNonNegative(field string, x float64) *Validate {
	if x < 0 {
		v.errs = append(v.errs, field+" must not be negative")
	}
	return v
}

// RequireRange appends msg when x is not within [lo,hi].
func (v *Validate) RequireRange(field string, x, lo, hi float64) *Validate {
	if x < lo || x > hi {
		v.errs = append(v.errs, field+" out of allowed range")
	}
	return v
}

// RequireEnum appends msg when v is not one of allowed.
func (v *Validate) RequireEnum(field, val string, allowed []string) *Validate {
	for _, a := range allowed {
		if a == val {
			return v
		}
	}
	v.errs = append(v.errs, field+" has invalid value '"+val+"'")
	return v
}

// RequireMatch appends msg when s does not match pattern.
func (v *Validate) RequireMatch(field, s, pattern string) *Validate {
	re, err := regexp.Compile(pattern)
	if err != nil {
		v.errs = append(v.errs, field+" has invalid validation pattern")
		return v
	}
	if !re.MatchString(s) {
		v.errs = append(v.errs, field+" has invalid format")
	}
	return v
}

// RequireLen appends msg when the rune-length of s is outside [min,max].
func (v *Validate) RequireLen(field, s string, min, max int) *Validate {
	n := utf8.RuneCountInString(s)
	if n < min || n > max {
		v.errs = append(v.errs, field+" length out of range")
	}
	return v
}

// Error assembles the accumulated errors into a single platform.Error with
// category ErrInvalid. Returns nil when there are no errors.
func (v *Validate) Error() error {
	if len(v.errs) == 0 {
		return nil
	}
	return Invalidf(strings.Join(v.errs, "; "))
}

// Has reports whether any validation error has been recorded.
func (v *Validate) Has() bool { return len(v.errs) > 0 }

// Messages returns a copy of the accumulated error messages.
func (v *Validate) Messages() []string {
	out := make([]string, len(v.errs))
	copy(out, v.errs)
	return out
}

// NonEmpty is a standalone helper that reports whether s contains non-space
// characters.
func NonEmpty(s string) bool { return strings.TrimSpace(s) != "" }

// IsOneOf reports whether v equals one of allowed.
func IsOneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}

// CleanID trims and uppercases an ID for stable lookups.
func CleanID(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// SnakeToUpper uppercases a snake_case identifier for display.
func SnakeToUpper(s string) string { return strings.ToUpper(s) }
