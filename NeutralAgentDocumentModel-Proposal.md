# Proposal: a format-neutral internal agent document model

Status: **idea, not scheduled.** Written 2026-09-19 while designing the Codex built-in harness
(run 20260919T074609Z-e6f7). Deliberately NOT adopted for that work; recorded so the reasoning is
not lost and so a later iteration can pick it up without re-deriving it.

## The observation that prompted this

While designing how a non-Markdown harness (Codex, whose agents are TOML files) enters the
Deployment pipeline, one fact turned out to drive everything: **MOSAIC has no format-neutral
representation of an agent.** The internal representation is the Markdown document itself.

`mosaic-common/docformat` parses a MOSAIC file into a document object, but that object is a view
over the original Markdown bytes: frontmatter fields plus a body in which managed regions are
addressable. `internal/transform` edits that document in place and then asks for its bytes back.
There is no intermediate "agent" object with fields like identity, description, model,
capabilities, instruction sections - or rather, there is, but it is spelled as "YAML frontmatter
keys plus Markdown region tags", and that spelling is the same object the tool writes to disk.

## Why it is like that (and it is not an accident)

The hard guarantee of the transform is that **every byte of an agent body outside a managed
region is identical to the source**. The cheapest and most bulletproof way to guarantee that is
to never re-render the text: keep the original bytes and surgically replace only the regions that
are MOSAIC-managed. A neutral model that parsed Markdown into structure and rendered it back
would have to reproduce the author's exact formatting - spacing, list style, code fences, line
endings - or the guarantee becomes "semantically equivalent", which is weaker and harder to test.

So the current design trades architectural purity for a very strong, very cheap correctness
property. That trade was a reasonable one while every harness used the same format.

## What the neutral model would look like

```
generic source (any format)  --parse-->   AGENT MODEL   --render-->  deployed file (any format)
                                          (identity, description, model,
                                           capabilities, provenance stamps,
                                           ordered body sections with
                                           ownership: source-owned /
                                           tool-managed / user-owned)
```

- Markdown-with-frontmatter becomes one renderer/parser pair among several, not the centre.
- Codex TOML becomes another. So would any future format.
- Transform would operate on the model (fields and sections), not on text.
- Drift detection, promote and retarget would compare models, not documents.

## Why it was not done for the Codex work

- It is a rewrite of a large shared component (`docformat` is roughly 17k lines including its
  test suite) that every tool in the repository depends on.
- It puts the byte-preservation guarantee at risk, and that guarantee is what makes users trust
  that MOSAIC will not mangle their agents.
- It delivers nothing a user can see. The Codex harness works either way.
- The risk profile is wrong for an iteration whose point is "add a harness".

## What was done instead, and why it is a stepping stone rather than a detour

The Codex design introduces a **format translator pair** (encode / decode) per agent file format,
selected by a field in the harness descriptor, applied at the single write seam and at every
deployed-file read seam. Markdown is itself a translator (an identity pair), so there is no
special case.

This matters for this proposal because it creates, for the first time, **an explicit and named
boundary between "what MOSAIC thinks an agent is" and "what the file on disk looks like"**. Today
that boundary is implicit and therefore unmovable.

If the neutral model is built later:

- The translators are exactly the place the new model plugs into. Their outer side (bytes on
  disk) does not change at all. Only their inner side changes, from "a docformat document" to
  "an agent model".
- Every call site - probe, update, promote, retarget, index scan, workspace scan - already goes
  through the translator, so none of them moves.
- The Markdown translator becomes the compatibility renderer, and the byte-preservation
  guarantee can be kept for Markdown specifically while other formats get a clean rendering path.

The alternative design that was rejected for Codex (serialization methods on the harness port)
would have made this proposal harder, not easier: the format logic would be spread across five
harness modules plus an out-of-process wire protocol, all of which would have to change again.

## If this is picked up

Questions to answer first, in roughly this order:

1. Is the byte-preservation guarantee kept for Markdown, relaxed to semantic equivalence for all
   formats, or kept only for user-owned regions?
2. What exactly is an agent, as a model? The frontmatter vocabulary and the region ownership
   classes already encode most of the answer and should be harvested, not reinvented.
3. Can the model be introduced behind the existing translator boundary one format at a time
   (Codex first, since it has no legacy deployed files in the wild), so the Markdown path
   migrates last and with the most evidence?
4. What proves it: a corpus test that every agent in the catalog round-trips byte-identically
   through the new model for every existing harness.
