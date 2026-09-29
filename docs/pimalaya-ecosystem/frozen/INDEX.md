# Frozen and deprecated (do not depend)

Per [pimalaya.org/ecosystem](https://pimalaya.org/ecosystem). Index-only; not cloned by default (`clone-manifest.yaml` tier `D_index_only`).

| id | status | replacement |
|----|--------|-------------|
| io-email | frozen | Protocol-direct clients (`io-imap`, `io-jmap`, `io-gmail`, …) |
| io-addressbook | frozen | Protocol-direct contacts clients |
| io-calendar | frozen | Protocol-direct calendar clients |
| io-process | deprecated | Per-store / per-tool process handling |
| io-keyring | deprecated | Third-party keyring CLIs documented by tools |
| io-fs | deprecated | Per-store filesystem coroutines |
| mimosa | deprecated | Retired with io-keyring |

Host architecture MUST NOT plan on aggregators in the first milestone.
