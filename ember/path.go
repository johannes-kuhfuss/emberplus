package ember

// Path is a Glow RELATIVE-OID represented as non-negative components.
type Path []int

// ParsePath validates and parses a dotted Glow path.
func ParsePath(value string) (Path, error) {
	components, err := parsePath(value)
	if err != nil {
		return nil, err
	}
	return Path(components), nil
}

// String returns the conventional dotted representation.
func (p Path) String() string { return formatPath([]int(p)) }

// Append returns a copy with component appended.
func (p Path) Append(component int) Path {
	out := append(Path(nil), p...)
	return append(out, component)
}

// MarshalText allows Path to be used naturally by text and JSON encoders.
func (p Path) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

// Number returns the final path component, if any.
func (p Path) Number() (int, bool) {
	if len(p) == 0 {
		return 0, false
	}
	return p[len(p)-1], true
}
