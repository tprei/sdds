# Description


# Test Layer

Name the lowest faithful layer for the changed behavior and why a lower layer cannot prove it (for example, parser-to-wire unit test, repository-to-SQLite, handler-to-OpenAPI, Compose-to-runtime, RustFS-to-object-store, or Expo-to-API):


# Tests / Validations

- [ ] `pnpm check`
- [ ] `pnpm smoke api` — assembled API image, migrations, routing, SQLite persistence, generated public client, endpoint set, public HTTP contract, or `services/embedding/**` changed
- [ ] `pnpm smoke rustfs` — object-store semantics, credentials/policy, readiness/bootstrap, private access, restart persistence, migration-without-media, or Compose media graph changed
- [ ] `pnpm smoke synthetics` — a critical user-visible Expo web journey, layout, spacing, responsive behavior, or appearance changed
- [ ] A changed Playwright spec names its user-visible boundary and why a lower layer cannot prove it
- [ ] A changed fake or fixture states the production contract and failure semantics it mirrors
- [ ] No assertion was duplicated at a higher layer and no moved test left a copy behind
- [ ] No oversized legacy file grew as the destination for unrelated behavior

Ok to leave unchecked with a short reason.
