package cli

func helpText() string {
	return "snapvector --capture --base64-stdout   emit display PNG as canonical JSON\n" +
		"snapvector --inject-svg <json> [--output-format=svg|png|jpg|pdf] [--copy-to-clipboard]\n" +
		"snapvector --version\n" +
		"snapvector --help\n"
}
