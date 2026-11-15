package agentformat

// DecodeAgentFile decodes deployed agent file bytes to the standard form for a
// read-only caller. It supplies the decode context itself (an agent artifact, no
// prior bytes, no refresh mode), so a read-only caller needs none of the write-side
// types to decode a file.
func DecodeAgentFile(t Translator, deployed []byte) ([]byte, error) {
	canonical, _, err := t.Decode(deployed, ArtifactContext{Kind: ArtifactAgent})
	return canonical, err
}
