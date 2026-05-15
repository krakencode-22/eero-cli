# Eero auth research notes

This CLI now treats eero authentication as two separate flows:

- Mobile API auth for device/profile/guest management at `https://api-user.e2ro.com`
- Web account auth for Amazon/account/subscription pages at `https://account.eero.com`

## Confirmed locally

- Amazon Login successfully authenticates the browser to `https://account.eero.com/`.
- The authenticated web account page shows the account email and network name.
- The eero web `session` cookie is `HttpOnly`, so page JavaScript cannot read it.
- The readable browser cookies are not enough to authenticate the CLI to `account.eero.com`.
- A same-origin browser fetch to `https://account.eero.com/` returns the authenticated account page.
- Obvious JSON-like web routes such as `/api/account`, `/account`, `/me`, `/user`, and `/networks` returned 404 HTML pages.
- The authenticated web context could not fetch `https://api-user.e2ro.com/2.2/account` or network device endpoints from the browser.
- A web account session token observed during research was rejected by the mobile API as `error.session.invalid`.
- In the official iOS app via iPhone Mirroring, the Amazon Login flow accepts the account email, offers passkey sign-in, and then requires local OS password/biometric approval before the app can complete login.
- After owner approval, the official iOS app reached the logged-in network home screen.
- The Mac CLI still reported `Status: Not logged in` afterward, confirming the iOS app session is not automatically available to the local CLI config.
- In the logged-in iOS app, `Settings > User permissions` shows an invite-admin flow for adding additional network admins.
- `Settings > Account settings` shows the Amazon-linked identity but did not expose an obvious control for adding legacy email/phone credentials to the owner account.

## Working CLI support

- `eero-cli login` keeps the legacy email/phone verification-code flow.
- `eero-cli login import-token [token]` validates and saves an existing mobile API session token.
- `eero-cli web open` opens the web account login page for Amazon/web sign-in.
- `eero-cli web account` validates a supplied `account.eero.com` Cookie header and prints the web account/network details.

## Boundary

The web account session is useful for proving Amazon/web login and reading the limited account page, but it is not currently evidence of device-management access. Device commands still require a valid mobile API `s` session token.

## Next live verification step

After owner approval in the official eero app, retry CLI auth checks and look for a safe way to obtain or validate a mobile API session token without storing Amazon/web cookies.

The preferred path is still current-account authentication: either the legacy email/phone verification flow succeeds for that account, or a valid current-account mobile API token is obtained through an authorized path and imported with `login import-token`.

Inviting a separate reachable admin identity can demonstrate that the CLI works against the mobile API, but it should be treated as an explicit fallback rather than the primary Amazon Login solution.

## Capture validation notes

Existing source and community clients corroborate `https://api-user.e2ro.com` as the unofficial mobile API endpoint, but they do not prove the exact Amazon Login handoff used by the current official app. To observe that handoff directly, use a physical iPhone/iPad with an HTTPS proxy such as Charles, Proxyman, or mitmproxy, or obtain a vendor-provided simulator-compatible eero `.app`. A normal App Store device `.ipa` is not enough for iOS Simulator.

If certificate pinning prevents decrypted capture, do not bypass it. Host-level DNS/SNI/timing evidence can still confirm whether the app touches `api-user.e2ro.com` after Amazon Login, but it cannot reveal exact JSON payloads or reusable session cookies.
