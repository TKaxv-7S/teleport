{{define "FormatCommand" -}}
{{if .FlagSummary}} {{.FlagSummary}}{{end -}}
{{range .Args}}{{if not .Hidden}} {{if not .Required}}[{{end}}{{if .PlaceHolder}}{{.PlaceHolder}}{{else}}<{{.Name}}>{{end}}{{if .Value|IsCumulative}}...{{end}}{{if not .Required}}]{{end}}{{end}}{{end -}}
{{end -}}

{{define "FormatCommands" -}}
{{$appName := .Name}}
{{range .FlattenedCommands -}}
## {{$appName}} {{.FullCommand}}

{{if not .Hidden -}}
  {{.FullCommand}}{{template "FormatCommand" .}}
{{.Help|Wrap 4}}

Flags:

|Flag|Description|
|---|---|
{{.Flags|FlagsToTwoColumns|FormatTwoColMarkdownTable}}
{{end -}}
{{end -}}
{{end -}}

{{define "FormatUsage" -}}
```code
$ {{.Name}}{{template "FormatCommand" .}}{{if .Commands}} <command> [<args> ...]{{end}}
```
{{if .Help}}
{{.Help|Wrap 0 -}}
{{end -}}

{{end -}}
---
title: {{.App.Name}} Reference
description: Provides a comprehensive list of commands, arguments, and flags for {{.App.Name}}.
---

This guide provides a comprehensive list of commands, arguments, and flags for
{{.App.Name}}.

{{template "FormatUsage" .App}}
{{if .Context.Flags -}}

Global flags:

|Flag|Description|
|---|---|
{{.Context.Flags|FlagsToTwoColumns|FormatTwoColMarkdownTable}}
{{end -}}
{{if .Context.Args -}}

Arguments:

|Argument|Description|
|---|---|
{{.Context.Args|ArgsToTwoColumns|FormatTwoColMarkdownTable}}
{{end -}}
{{if .App.Commands -}}

{{template "FormatCommands" .App}}
{{end -}}

