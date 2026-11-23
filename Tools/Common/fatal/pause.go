package fatal

// pause prints PausePrompt and waits for Enter (a newline), end of input or a read
// error. Console input typed before the prompt is discarded first.
func (r *Reporter) pause() {
	discardPendingInput(r.cfg.Stdin)
	r.print(PausePrompt)
	buf := make([]byte, 1)
	for {
		n, err := r.cfg.Stdin.Read(buf)
		if n > 0 && buf[0] == '\n' {
			return
		}
		if err != nil {
			return
		}
	}
}
