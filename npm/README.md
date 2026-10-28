# @agentsdance/aigit

AI-powered Git commit messages using your current coding agent or an LLM provider.

```sh
npm install -g @agentsdance/aigit
aigit version
aigit commit
```

You can also run it without a global installation:

```sh
npx @agentsdance/aigit commit
```

Requires Node.js 20 or newer and Git. The installer downloads the prebuilt aigit
binary for the npm package's exact version and verifies its SHA256 checksum. No Go
toolchain is required. Supported platforms are macOS (Apple Silicon and Intel),
Linux x64, and Windows x64. Installation requires access to GitHub release assets.
If installation scripts are disabled, the first invocation downloads the binary.

Update with:

```sh
npm install -g @agentsdance/aigit@latest
```

npm manages updates for this installation, so the CLI's built-in source upgrade
prompt is disabled.

To generate commit messages with your coding agent's existing session, install the
agent skill separately:

```sh
npx skills add agentsdance/aigit --skill aigit
```

See the [full documentation](https://github.com/agentsdance/aigit#readme) for provider
configuration, staged-change preparation, previews, and committing agent responses.
