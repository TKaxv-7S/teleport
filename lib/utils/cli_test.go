/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestUserMessageFromError(t *testing.T) {
	// Behavior is different in debug
	defaultLogger := slog.Default()

	var leveler slog.LevelVar
	leveler.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: &leveler})))
	t.Cleanup(func() {
		slog.SetDefault(defaultLogger)
	})

	tests := []struct {
		comment   string
		inError   error
		outString string
	}{
		{
			comment:   "outputs x509-specific unknown authority message",
			inError:   trace.Wrap(x509.UnknownAuthorityError{}),
			outString: "WARNING:\n\n  The proxy you are connecting to has presented a",
		},
		{
			comment:   "outputs x509-specific invalid certificate message",
			inError:   trace.Wrap(x509.CertificateInvalidError{}),
			outString: "WARNING:\n\n  The certificate presented by the proxy is invalid",
		},
		{
			comment:   "outputs user message as provided",
			inError:   trace.Errorf("bad thing occurred"),
			outString: "\x1b[31mERROR: \x1b[0mbad thing occurred",
		},
	}

	for _, tt := range tests {
		message := UserMessageFromError(tt.inError)
		require.Contains(t, message, tt.outString)
	}
}

// TestEscapeControl tests escape control
func TestEscapeControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  string
		out string
	}{
		{
			in:  "hello, world!",
			out: "hello, world!",
		},
		{
			in:  "hello,\nworld!",
			out: `"hello,\nworld!"`,
		},
		{
			in:  "hello,\r\tworld!",
			out: `"hello,\r\tworld!"`,
		},
	}

	for i, tt := range tests {
		require.Equal(t, tt.out, EscapeControl(tt.in), fmt.Sprintf("test case %v", i))
	}
}

// TestAllowWhitespace tests escape control that allows (some) whitespace characters.
func TestAllowWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  string
		out string
	}{
		{
			in:  "hello, world!",
			out: "hello, world!",
		},
		{
			in:  "hello,\nworld!",
			out: "hello,\nworld!",
		},
		{
			in:  "\thello, world!",
			out: "\thello, world!",
		},
		{
			in:  "\t\thello, world!",
			out: "\t\thello, world!",
		},
		{
			in:  "hello, world!\n",
			out: "hello, world!\n",
		},
		{
			in:  "hello, world!\n\n",
			out: "hello, world!\n\n",
		},
		{
			in:  string([]byte{0x68, 0x00, 0x68}),
			out: "\"h\\x00h\"",
		},
		{
			in:  string([]byte{0x68, 0x08, 0x68}),
			out: "\"h\\bh\"",
		},
		{
			in:  string([]int32{0x00000008, 0x00000009, 0x00000068}),
			out: "\"\\b\"\th",
		},
		{
			in:  string([]int32{0x00000090}),
			out: "\"\\u0090\"",
		},
		{
			in:  "hello,\r\tworld!",
			out: `"hello,\r"` + "\tworld!",
		},
		{
			in:  "hello,\n\r\tworld!",
			out: "hello,\n" + `"\r"` + "\tworld!",
		},
		{
			in:  "hello,\t\n\r\tworld!",
			out: "hello,\t\n" + `"\r"` + "\tworld!",
		},
	}

	for i, tt := range tests {
		require.Equal(t, tt.out, AllowWhitespace(tt.in), fmt.Sprintf("test case %v", i))
	}
}

func TestUpdateAppUsageTemplate(t *testing.T) {
	makeApp := func(usageWriter io.Writer) *kingpin.Application {
		app := InitCLIParser("TestUpdateAppUsageTemplate", "some help message")
		app.UsageWriter(usageWriter)
		app.Terminate(func(int) {})

		app.Command("hello", "Hello.")

		create := app.Command("create", "Create.")
		create.Command("box", "Box.")
		create.Command("rocket", "Rocket.")
		return app
	}

	tests := []struct {
		name           string
		inputArgs      []string
		outputContains string
	}{
		{
			name:      "command width aligned for app help",
			inputArgs: []string{},
			outputContains: `
Commands:
  help          Show help.
  hello         Hello.
  create box    Box.
  create rocket Rocket.
`,
		},
		{
			name:      "command width aligned for command help",
			inputArgs: []string{"create"},
			outputContains: `
Commands:
  create box    Box.
  create rocket Rocket.
`,
		},
		{
			name:      "command width aligned for unknown command error",
			inputArgs: []string{"unknown"},
			outputContains: `
Commands:
  help          Show help.
  hello         Hello.
  create box    Box.
  create rocket Rocket.
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("help flag", func(t *testing.T) {
				var buffer bytes.Buffer
				app := makeApp(&buffer)
				args := append(tt.inputArgs, "--help")
				UpdateAppUsageTemplate(app, args)

				app.Usage(args)
				require.Contains(t, buffer.String(), tt.outputContains)
			})

			t.Run("help command", func(t *testing.T) {
				var buffer bytes.Buffer
				app := makeApp(&buffer)
				args := append([]string{"help"}, tt.inputArgs...)
				UpdateAppUsageTemplate(app, args)

				// HelpCommand is triggered on PreAction during Parse.
				// See kingpin.Application.init for more details.
				_, err := app.Parse(args)
				require.NoError(t, err)
				require.Contains(t, buffer.String(), tt.outputContains)
			})
		})
	}
}

func TestPrintCLIDocs(t *testing.T) {
	tests := []struct {
		name     string
		makeApp  func() *kingpin.Application
		expected string // The @ character is replaced with a backtick
	}{
		{
			name: "subcommand flags and global flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("hello", "Hello.")
				create := app.Command("create", "Create.")
				create.Flag("name", "The name of the resource").Default("myresource").String()
				createRocket := create.Command("rocket", "Rocket.")
				createRocket.Flag("launch", "Whether to launch the Rocket").Bool()
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp create rocket

Rocket.

Usage:

@@@code
$ myapp create rocket [<flags>]
@@@

Flags:

|Flag|Default|Description|
|---|---|---|
|@--[no-]launch@|@false@|Whether to launch the Rocket|

## myapp hello

Hello.

Usage:

@@@code
$ myapp hello
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

`,
		},
		{
			name: "multiple main command flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Flag("verbosity", "Verbosity level.").Default("3").Int()
				app.Flag("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|
|@--verbosity@|@3@|Verbosity level.|
|@--[no-]dry-run@|@false@|Whether to use dry-run mode|

`,
		},
		{
			name: "multiple subcommand flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				create := app.Command("create", "Create a resource.")
				create.Flag("verbosity", "Verbosity level.").Default("3").Int()
				create.Flag("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp create

Create a resource.

Usage:

@@@code
$ myapp create [<flags>]
@@@

Flags:

|Flag|Default|Description|
|---|---|---|
|@--verbosity@|@3@|Verbosity level.|
|@--[no-]dry-run@|@false@|Whether to use dry-run mode|

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

`,
		},
		{
			name: "multiple sub-command args",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				create := app.Command("create", "Create.")
				create.Arg("verbosity", "Verbosity level.").Default("3").Int()
				create.Arg("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp create

Create.

Usage:

@@@code
$ myapp create [<verbosity>] [<dry-run>]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|verbosity|@3@ (optional)|Verbosity level.|
|dry-run|@false@ (optional)|Whether to use dry-run mode|

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

`,
		},
		{
			name: "sub-command order",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("create", "Create a resource.")
				app.Command("validate", "Validate the config.")
				app.Command("connect", "Connect to a server.")
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp connect

Connect to a server.

Usage:

@@@code
$ myapp connect
@@@

## myapp create

Create a resource.

Usage:

@@@code
$ myapp create
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

## myapp validate

Validate the config.

Usage:

@@@code
$ myapp validate
@@@

`,
		},
		{
			name: "level-3 command order",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				mfa := app.Command("mfa", "Manage MFA resources.")
				mfa.Command("add", "Add an MFA device.")
				app.Command("create", "Create a resource")
				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp create

Create a resource

Usage:

@@@code
$ myapp create
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

## myapp mfa add

Add an MFA device.

Usage:

@@@code
$ myapp mfa add
@@@

`,
		},
		{
			name: "empty arg",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("kubectl", "Proxy kubectl commands.")
				kubectl := app.Command("kubectl", "Proxy kubectl commands.").Interspersed(false)
				// This hack is required in order to accept any args for tsh kubectl.
				kubectl.Arg("", "").StringsVar(new([]string))

				return app
			},
			expected: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

This is the main CLI tool.

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

## myapp kubectl

Proxy kubectl commands.

Usage:

@@@code
$ myapp kubectl [args...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|args|none (optional)|Arbitrary arguments|

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.makeApp()

			var buffer bytes.Buffer
			app.Terminate(func(int) {})
			PrintCLIDocs(&buffer, app)
			require.Equal(t, strings.ReplaceAll(tt.expected, "@", "`"), buffer.String())
		})
	}
}
