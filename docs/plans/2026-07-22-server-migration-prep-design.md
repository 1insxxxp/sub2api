# Server Migration Preparation Design

## Goal

Prepare the new server at `95.169.9.146` for a later migration from the
existing server without copying production data, changing DNS, or stopping
the existing services.

## Operating System

Reinstall the new, currently empty server with Ubuntu 22.04 LTS so its base
system matches the existing Ubuntu 22.04 server. Keep SSH on port 22.

## Base Environment

Install and enable:

- Docker Engine from Docker's official Ubuntu repository
- Docker Compose plugin
- Nginx
- rsync, curl, unzip, ca-certificates, and time synchronization tools
- UFW with inbound access for SSH, HTTP, HTTPS, and TCP port 18600

Create `/opt/migration` for staged transfer files and `/opt/backups` for
pre-cutover backups.

## Safety Boundaries

- Do not modify or stop services on the existing server.
- Do not copy application or database data during preparation.
- Do not start duplicate production applications.
- Do not change DNS records.
- Keep password-based root access until migration access has been verified;
  SSH hardening is a separate follow-up operation.

## Verification

Confirm that the new server runs Ubuntu 22.04, Docker and Compose respond,
Nginx configuration validation succeeds, UFW contains the expected rules,
time synchronization is active, required directories exist, and no failed
systemd services remain.
