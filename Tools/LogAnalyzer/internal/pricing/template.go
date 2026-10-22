package pricing

// pricingTemplate is the documentation written at the head of a pricing file that
// LogAnalyzer creates because it was absent. It consists only of comment and blank
// lines, so a file holding just the template loads as an empty table. Later writes
// keep it because they preserve the file's leading comment block.
const pricingTemplate = `# MosaicLogAnalyzer pricing configuration
#
# Each entry under "models:" prices one model. Every model has a standard rate
# set with exactly these five rates:
#
#   input           regular input tokens
#   cache_read      tokens read from the prompt cache
#   cache_write_5m  tokens written to the 5-minute cache tier
#   cache_write_1h  tokens written to the 1-hour cache tier
#   output          output tokens
#
# All rates are USD per million tokens (1,000,000 tokens). Write each rate as a
# quoted string, for example "3.00", so YAML does not parse it as a float.
#
# A model may also have one optional "long_context" block: a "threshold" (tokens
# per request) plus its own full rate set with the same five rates. A model has at
# most one long-context tier.
#
# When a usage record has no cache-write tier split, the cache write is assumed to
# use the 5-minute tier (cache_write_5m) as the fallback.
#
# Example entry without a long-context tier:
#
#   example-model:
#     input: "3.00"
#     cache_read: "0.30"
#     cache_write_5m: "3.75"
#     cache_write_1h: "6.00"
#     output: "15.00"
#
# Example entry with a long-context tier:
#
#   example-long-context-model:
#     input: "3.00"
#     cache_read: "0.30"
#     cache_write_5m: "3.75"
#     cache_write_1h: "6.00"
#     output: "15.00"
#     long_context:
#       threshold: 200000
#       input: "6.00"
#       cache_read: "0.60"
#       cache_write_5m: "7.50"
#       cache_write_1h: "12.00"
#       output: "22.50"

`
