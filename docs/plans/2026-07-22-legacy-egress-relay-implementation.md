# Legacy Server OpenAI Egress Relay Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Route the new production application's OpenAI upstream requests through the old server's supported US IPv4 without moving application data or inbound traffic.

**Architecture:** Run a minimal authenticated forward proxy on the old server and restrict its listener with the old server firewall to the new server IPv4. Register that proxy in Sub2API, associate OpenAI accounts with it, and verify the application reaches OpenAI through the old IPv4.

**Tech Stack:** Ubuntu, systemd, UFW, Docker Compose, PostgreSQL, Sub2API proxy support

---

### Task 1: Baseline and Safety Checks

**Files:**
- Inspect: `/opt/sub2api/docker-compose.yml` on the new server
- Inspect: `/etc/ufw/user.rules` on the old server

1. Verify both hosts are reachable and record current container health.
2. Verify old server direct OpenAI requests return 401 and new server requests return regional 403.
3. Inspect available proxy packages and choose the smallest supported daemon.
4. Confirm an unused private proxy port and new-to-old connectivity.

### Task 2: Deploy Restricted Proxy

**Files:**
- Create: `/etc/3proxy/3proxy.cfg` or the selected daemon equivalent on the old server
- Create: `/etc/systemd/system/3proxy.service` only if the package does not provide one
- Modify: old server UFW rules

1. Install the proxy package from the Ubuntu repository.
2. Generate a unique proxy credential without displaying it in logs.
3. Bind the proxy listener and permit only `166.88.36.252` in both proxy ACLs and UFW.
4. Start and enable the service.
5. Verify an unauthorized local-source simulation is denied where practical.

### Task 3: Verify the Egress Path

1. From the new server, request an IP echo endpoint through the proxy.
2. Expect the observed IPv4 to equal `176.122.172.176`.
3. Request OpenAI through the proxy without credentials.
4. Expect HTTP 401 rather than `unsupported_country_region_territory` 403.

### Task 4: Connect Sub2API Accounts

**Files:**
- Modify: `proxies` and `accounts.proxy_id` rows in the new PostgreSQL database

1. Back up the affected proxy and account association rows.
2. Insert or update one dedicated active proxy record using the generated credential.
3. Associate active OpenAI accounts with the proxy in one transaction.
4. Confirm the account repository loads the proxy relationship.
5. Restart only the application container if cache invalidation requires it.

### Task 5: End-to-End Verification

1. Verify all three containers remain healthy.
2. Run one known OpenAI OAuth account connection test through the application.
3. Confirm the upstream response is no longer the regional HTML 403 page.
4. Confirm normal gateway requests still return successful responses.
5. Record exact rollback SQL and the proxy service removal commands.

### Task 6: Rollback on Failure

1. Restore the backed-up account proxy associations in one transaction.
2. Stop and disable the old-server proxy service.
3. Remove only the dedicated UFW proxy rule.
4. Verify the new application remains healthy after rollback.
