# Offline GeoIP test fixture

`GeoIP2-City-Test.mmdb` is the small, generated public fixture from MaxMind's
MaxMind-DB repository. It contains synthetic test records, not customer data.

- Source: https://github.com/maxmind/MaxMind-DB/blob/0ee49278e35fd7bdf341ac236edbba20de1aceb9/test-data/GeoIP2-City-Test.mmdb
- Fixed upstream commit: `0ee49278e35fd7bdf341ac236edbba20de1aceb9`
- SHA-256: `7ecd1a263834096e3948b459713102ef350412a9f18367b4a88efec018153004`
- Upstream copyright: MaxMind, Inc. (2013–2026).
- Upstream offers Apache 2.0 or MIT at the recipient's option. This fixture is
  redistributed under the MIT option; the unchanged license is in `LICENSE-MIT`.
  The licensing statement is in the upstream repository's `README.md` at the
  same commit.

The default unit tests read this file to verify real MMDB decoding, IPv4-mapped
IPv6 handling, and unmatched addresses without network access or credentials.
The fixture must not be installed as the application's production GeoIP database.
