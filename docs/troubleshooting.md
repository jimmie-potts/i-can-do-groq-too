# Troubleshooting

## Codex Desktop browser control fails from WSL

**Observed error:**

```text
Mcp error: -32602: js: codex/sandbox-state-meta:
sandboxCwd is not a local file URI: file:///home/jimmie/projects/i-can-do-groq-too
```

### What it means

The Codex task and repository run in WSL with a Linux workspace path such as `/home/...`. Browser
control is hosted by a Windows-side `node_repl.exe`. Codex sends the task workspace as
`file:///home/...`; the Windows process cannot interpret that URI as a local Windows path and
rejects it before any JavaScript or browser initialization runs.

This is not caused by this repository, Chrome page content, provider configuration, Git, or the
public ChatGPT share. It occurs before browser-client code executes.

The matching upstream report is
[openai/codex#29639](https://github.com/openai/codex/issues/29639). It remained open when this setup
was validated on 2026-07-31. Current reports also identify a separate Desktop eligibility signal,
`reason=wsl-disabled`, tracked in
[openai/codex#25301](https://github.com/openai/codex/issues/25301), so translating `sandboxCwd` may be
necessary but not sufficient for the in-app browser.

### Practical workaround

For a browser-dependent Codex task, use a native Windows workspace and run that task with the
Windows agent/runtime. Both sides then use a Windows-local file URI.

For read-only public content, use a non-browser retrieval path or attach/export the source as
Markdown or text. Manual use of an external Chrome window also remains available, but it does not
repair Codex's automated WSL bridge.

### Approaches that do not fix the root cause

- changing only one shell command's working directory;
- editing a generated MCP `cwd` while Codex still sends WSL `sandboxCwd` metadata;
- opening the repository through an unverified WSL UNC path;
- toggling full access or feature flags;
- reinstalling the Chrome extension; or
- changing repository files.

The durable product fix must translate WSL paths into the path namespace of a Windows MCP process,
for example `/mnt/c/...` to a Windows drive URI and `/home/...` to a supported WSL UNC/file URI, and
must also account for the Desktop WSL browser availability gate.

### Sharing long ChatGPT conversations

A public share link is workable and was sufficient for this bootstrap after decoding its
server-rendered public payload. The most reliable option for future long project context is an
attached Markdown or text export because it is directly readable, reviewable, and versionable and
does not depend on browser automation.

## GitHub SSH push rejects a WSL system configuration include

**Observed error:**

```text
Bad owner or permissions on /etc/ssh/ssh_config.d/20-systemd-ssh-proxy.conf
fatal: Could not read from remote repository.
```

### What it means

OpenSSH validates system configuration ownership before using a key. In this WSL environment,
`/etc/ssh/ssh_config.d/20-systemd-ssh-proxy.conf` is a symlink owned by `nobody:nogroup`, so SSH
rejects the configuration before contacting GitHub. The GitHub key remains valid: bypassing that
system configuration produced GitHub's successful-authentication response.

### Repository-local workaround

This checkout uses the following local Git setting:

```bash
git config --local core.sshCommand "ssh -F /dev/null"
```

It applies only to this repository and leaves global Git/SSH configuration and system files
unchanged. It skips the rejected system configuration while retaining OpenSSH's default key and
known-host behavior. If a repository requires a custom key or proxy, point `-F` at a separately
reviewed user-owned SSH configuration instead.

The system-level alternative is to repair the include's ownership and permissions through the WSL
distribution's package/system administration path. That requires administrator intent and should
not be performed automatically by a repository setup task.
