# VisorCorpus

Go toolkit + library: structured adversarial corpora for LLM/RAG safety + quality testing. Prompt injection, KB exfiltration, jailbreak, system-prompt probing. CI/CD-ready.

## Language
Go

## Build & Run
```
go build ./cmd/visorcorpus
go build ./cmd/attack-sim
go build ./cmd/visorforge
go build ./cmd/visorfail
go build ./cmd/corpus-dump
go test ./...
```

## Claude Code Notes
- Check README for full CLI surface, library API, and CI workflow examples
- Follow existing code style and patterns in `pkg/corpus/`
- Run tests before committing changes
- Sample corpora live in `examples/` — schema reference for the JSON output
- Built with [Claude Code](https://claude.ai/code)
