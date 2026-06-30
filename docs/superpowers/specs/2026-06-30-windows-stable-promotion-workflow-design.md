# Windows Stable Promotion Workflow Design

## Goal

Promote an already built and Windows-validated Maka Browser release from
`test/<version>` to `stable/<version>` through a manually approved GitHub
Actions workflow. Promotion must not rebuild or replace the tested artifacts.

## Scope

- Add a manually dispatched `Windows Stable Promotion` workflow.
- Reuse the existing Windows self-hosted runner and release SSH secrets.
- Harden `tools/release/promote-windows-release.ps1` so remote shell scripts are
  transferred as LF-only UTF-8 files and executed by remote `bash`.
- Verify the stable manifest, update ZIP, and installer through the HTTP release
  endpoint after promotion.
- Keep the previous stable alias active when any pre-promotion validation fails.

The workflow does not build packages, modify application code, deploy the
1688shopManager server, or delete historical releases.

## Promotion Flow

1. An operator dispatches the workflow with a target version.
2. The workflow validates the version input and confirms that
   `test/<version>` exists.
3. The remote promotion script verifies required files and their SHA256
   sidecars inside the test release.
4. The script copies the complete tested directory to a temporary stable
   directory.
5. It verifies the copied ZIP and manifest before atomically renaming the
   temporary directory to `stable/<version>`.
6. It atomically publishes stable-root aliases for the ZIP and installer.
7. Only after the payload aliases exist does it atomically replace
   `stable/app-update-stable.json`; its relative package URL therefore remains
   valid throughout the switch.
8. The workflow downloads the stable manifest and validates its package URL and
   SHA256, then checks the installer URL.

## Idempotency and Failure Handling

- If `stable/<version>` already exists and matches the requested release, the
  promotion step reports that state instead of overwriting it.
- A conflicting or incomplete existing stable directory fails closed.
- Temporary remote scripts and temporary stable directories are removed on
  failure.
- The stable alias is written to a temporary file and renamed only after all
  artifact checks pass.
- Network, SSH, hash, or HTTP verification failures make the workflow fail; no
  successful release claim is made.

## Security and Ownership

- SSH credentials remain GitHub repository secrets and are written only to a
  temporary runner file with restricted permissions.
- GitHub Actions provides the human-controlled production gate.
- The release server remains the source of truth for stable release files.
- No frontend, backend business logic, RBAC, database, or
  `server/data/app.db` changes are involved.

## Verification

Static verification must check:

- the promotion workflow is manual and does not invoke the build pipeline;
- the workflow passes only the version into the promotion script;
- the PowerShell script does not send multiline Bash directly as an SSH
  command argument;
- each embedded Bash script passes `bash -n`;
- the stable alias update occurs after test and copied artifact verification.

Release verification for version `1.1.23` must confirm:

- the GitHub Actions promotion run succeeds;
- the stable manifest reports version `1.1.23`;
- the ZIP URL is reachable and matches the manifest SHA256;
- the installer URL is reachable;
- the test release remains available.
