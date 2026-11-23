package gitlog

func ParseHeader(line string) error {
	_, err := parseHeader(line)

	return err
}

func ParseChurnHeader(token string) error {
	_, _, err := parseChurnHeader(token)

	return err
}
