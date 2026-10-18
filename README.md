# tflint-ruleset-tofucompat

Checks Terraform configuration for constructs a given OpenTofu release cannot
run, and says how to fix each one. Where the fix is mechanical (for example
`convert()` to `tonumber()`, or deleting an output's `type`), it can apply it.

It ships two ways, built from one rule library:

- a **tflint plugin**, for teams that already run tflint;
- a **standalone CLI**, `tofucompat`, which also checks what tflint cannot
  show a plugin: `.tftest.hcl` test files, `.tofu` files, `.tfquery.hcl` and
  Stacks files, and `import` blocks in child modules.

These are the same rules [runners.io](https://www.runners.io) applies when
OpenTofu runs your Terraform configuration: at upload, and as a commit status
on a pull request. Running them yourself gives you the same answer before
you push.

Each rule records the first OpenTofu release that supports its construct. That
version was measured by running a must-flag and a clean fixture through real
OpenTofu binaries from 1.6 to 1.13, not read from changelogs. You choose the
OpenTofu version to check against. The default is the latest.

## The tflint plugin

Add this to `.tflint.hcl`:

```hcl
plugin "tofucompat" {
  enabled = true
  version = "0.1.0"
  source  = "github.com/runners-io/tflint-ruleset-tofucompat"

  # Optional. "latest" (the default), a line such as "1.10", or a full version.
  # opentofu_version = "1.10"

  signing_key = <<-KEY
  -----BEGIN PGP PUBLIC KEY BLOCK-----

  mQINBGrU2qABEACacJB6Bf328aiE23iMao0hymZjsSTIZZtAQjjaVCM9Zs4psJ/e
  u3IYCvvR1LmkWFeISN5gjFserJ55IMtsPitFJHMa6aa83s7gKM1jXnUJDOdKEmS/
  R0yOu3Fog9TjcOTDdOpJ8ZhjwD4cq/zfGHDfkaWBbTeRy4jbGETIVsoEa7Yehosp
  9Wy4o4HjVkgcgb8j7JwzZBVJAbAuRGLeX7GumW60+El0T+7DtgpC7eixcR9QM7dd
  p0XqQ4WpID8vdI5X01tlFfqqF4etDhoRLtNIttCAS1Qxbi70049qIlI8XCz8+fBQ
  TjBbVkJ930JakfK59g1eWTTQ0oOMMSf2jW/kgdbz/Kffhe7MCmBUhNDYnzyTMXu2
  9To3KkYZjLR8APHG6FslGKP+kRLEq1ZUUdrkJX5bA4ib91LFEiOmVEP9R9LfThwk
  EZW2LCBOrbiQlh6B1qdn8VaJ3LrvWFc3fNj8TxfqFaeqJdAXyLw4A62YMadAeKF+
  vfRnB3rDf9dh/+3m4S+gCJkuJuy2d6bnOzlDQHLFExpoVZWb0enrS+9Y5cFlfHkR
  QUgludq1T6Rc9erRNlXL2p3YZNukihCy6eZGZlRRyOu848b7ZVuBOskL0K6jcqm0
  6lCVXuNFJj+gnqtu4RP+ZIYcZPKAWGdogw0w+OV3ZmYi6oP4UMFI7fPe3wARAQAB
  tDRydW5uZXJzLmlvIHRmbGludC1ydWxlc2V0LXRvZnVjb21wYXQgcmVsZWFzZSBz
  aWduaW5niQJsBBMBCABXFiEE+WmO+6gnQ6PRnqAuy91QjRTg0VkFAmrU2qAbFIAA
  AAAABAAObWFudTIsMi41KzEuMTIsMCwzAhsDBQsJCAcCAiICBhUKCQgLAgQWAgMB
  Ah4HAheAAAoJEMvdUI0U4NFZHGEP9jw9j8aUZWHPjLXHGnZE84n+5C3j2GzxqT8F
  DP1YH/atb3r5KUG2SVDzNu8h4ILXIEi94+/Fn8R0U3lYaUr4U4myWi3sshbj8foS
  py0nr/ioWV344R2Tu2tUl1BQeMmwKprCCt1xiZcKrHLlSteW7wF2Vxo1XgddVhKc
  x1K+2DPRO2cNDIXIClwoWPBiFdeCp+8iQdVRD12M0VfMo2aN+P7vFWOdtogDuHMw
  xP9Bb2aUSE5IsNw3CbPJIrtI5MMCwVySzRJUKxNDdELq2kcreDKj8Sil2tSYkbLM
  XXeyA4+yl9J9qKxKHUlVcru8eVq0QK5KbSrYxnUaYuO3zWKBLyL1oSrowOq3P1Qf
  E9Ze2NPWkxKX4eidtn61XoPVsrhzkbDPieb6cjzIUyVfzwVbwTY/D5xrRzeXCq3x
  YWhJkGX4XzEgLgEEcMsOoekGH1eDIqWwIoORaw71xDYXml5hxyCVbMy7P9HjjSED
  XNw2KuZZS2uLprnTEXg2tDfD2eRFdr087V6wkINhVvfgnFnRM9IYTNjkQHtPxmVm
  aYBXWJJeJVei1adRNVbU4QtV2GNwqrZW9fEKstRA1zpebp8R2VP/qHl3fY0qzVEU
  ttNwyxBpSEOQT5KHMB1AH0xZzWA04MDWHys08h+Zd1Q3ATohlOOF0oA24LJnTiyU
  nF+I2J8=
  =TX+u
  -----END PGP PUBLIC KEY BLOCK-----
  KEY
}
```

Then:

```sh
tflint --init     # downloads the plugin and verifies its signature
tflint            # reports findings
tflint --fix      # applies the mechanical fixes
```

`tflint --init` refuses the download unless `checksums.txt.sig` verifies
against `signing_key`, and the plugin's zip matches its line in
`checksums.txt`. The same public key is in this repository as
[`signing-key.asc`](signing-key.asc). Its fingerprint is:

```
F969 8EFB A827 43A3 D19E  A02E CBDD 508D 14E0 D159
```

Each rule is a tflint rule with the same id, and it can be turned off the
usual way:

```hcl
rule "tofucompat_removed_without_lifecycle" {
  enabled = false
}
```

## The CLI

Download `tofucompat_<os>_<arch>.zip` for your platform from the
[latest release](https://github.com/runners-io/tflint-ruleset-tofucompat/releases/latest),
along with `checksums.txt` and `checksums.txt.sig`, and verify them:

```sh
gpg --import signing-key.asc
gpg --verify checksums.txt.sig checksums.txt
sha256sum --check --ignore-missing checksums.txt   # macOS: shasum -a 256 --check --ignore-missing
unzip tofucompat_linux_amd64.zip
```

Or build it from source with Go:

```sh
go install github.com/runners-io/tflint-ruleset-tofucompat/cmd/tofucompat@v0.1.0
```

Usage:

```sh
tofucompat [flags] [DIR ...]          # DIR defaults to ., and is checked recursively

tofucompat .                                   # against the latest OpenTofu
tofucompat -opentofu-version 1.10 infra/       # against OpenTofu 1.10
tofucompat -fix .                              # apply the mechanical fixes, then report what is left
tofucompat -format json .                      # machine-readable findings
tofucompat -list-rules -opentofu-version 1.8   # the rules that apply to a version
```

| flag | meaning |
|---|---|
| `-opentofu-version` | `latest` (default), a line such as `1.10`, or a full version |
| `-fix` | apply every mechanical fix in place |
| `-format` | `text` (default), `json`, or `github` (GitHub Actions annotations) |
| `-strict` | fail on warnings too |
| `-disable` | comma-separated rule ids to leave out |
| `-list-rules` | print the rules that apply to `-opentofu-version`, and exit |
| `-version` | print the version, and exit |

Exit status:

- `0` means no error finding (warnings do not fail the run unless you pass `-strict`).
- `1` means at least one error finding.
- `2` means the check could not run, including when it found no Terraform file to check. "Found nothing" and "read nothing" are different answers.

## In GitHub Actions

This repository is also a composite action. It builds the CLI from the source
at the ref you pin, so what runs is exactly that source. Pin it by commit SHA:

```yaml
- uses: actions/checkout@<sha>
- uses: runners-io/tflint-ruleset-tofucompat@<sha> # v0.1.0
  with:
    opentofu-version: latest   # or "1.10"
    working-directory: .
    strict: "false"
    format: github             # findings become annotations on the pull request
```

An error finding fails the step. A warning does not, unless `strict` is
`"true"`.

## Rules

`tofucompat -list-rules` prints every rule that applies to a version, with its
severity and whether it has a mechanical fix. Each finding links to its section of
the [OpenTofu compatibility reference](https://www.runners.io/docs/infrastructure-runs/opentofu-compatibility).

## Where the source lives

This repository is published from the runners.io monorepo, where the same rule
library runs inside the runners.io Terraform engine. Its contents are that
directory, byte for byte, and a daily check there fails when they differ. So a
change made only here would be overwritten by the next publish.

Issues and pull requests are welcome. We port an accepted pull request to the
monorepo and publish it from there.

## Licence

[MPL-2.0](LICENSE).
