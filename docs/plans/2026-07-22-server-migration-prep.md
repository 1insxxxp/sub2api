# Server Migration Preparation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prepare `95.169.9.146` as an Ubuntu 22.04 Docker and Nginx host for a later production migration.

**Architecture:** Reinstall the empty destination to match the source OS, then apply a minimal host baseline over SSH. Production applications, data, certificates, and DNS remain unchanged until a separate migration phase.

**Tech Stack:** Ubuntu 22.04 LTS, Docker Engine, Docker Compose plugin, Nginx, UFW, systemd-timesyncd, rsync

---

### Task 1: Reinstall the Destination OS

**Files:** None

1. In KiwiVM, open `Install new OS` for `95.169.9.146`.
2. Select Ubuntu 22.04 64-bit and start the reinstall.
3. Record the generated root password and wait until the server reports Running.
4. Verify SSH connectivity with `ssh -4 -B en1 root@95.169.9.146`.
5. Verify `source /etc/os-release && test "$VERSION_ID" = "22.04"` succeeds.

### Task 2: Update the Base System

**Files:** None

1. Run `apt-get update`.
2. Run `DEBIAN_FRONTEND=noninteractive apt-get -y upgrade`.
3. Install `ca-certificates curl gnupg rsync unzip ufw nginx`.
4. Enable time synchronization with `timedatectl set-ntp true`.

### Task 3: Install Docker Engine

**Files:**
- Create: `/etc/apt/keyrings/docker.asc`
- Create: `/etc/apt/sources.list.d/docker.list`

1. Add Docker's official Ubuntu signing key.
2. Add the official stable repository using the detected architecture and Ubuntu codename.
3. Install `docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin`.
4. Enable and start Docker.
5. Verify `docker version` and `docker compose version` succeed.

### Task 4: Configure the Host Baseline

**Files:**
- Create: `/opt/migration/`
- Create: `/opt/backups/`

1. Create the two directories with owner `root:root` and mode `0750`.
2. Enable and start Nginx.
3. Allow OpenSSH, HTTP, HTTPS, and TCP 18600 through UFW.
4. Enable UFW non-interactively.
5. Verify `nginx -t` succeeds and inspect `ufw status verbose`.

### Task 5: Final Verification

**Files:** None

1. Verify OS, CPU, memory, disk, and time synchronization.
2. Verify Docker, containerd, Nginx, and SSH are active.
3. Verify no systemd units are failed.
4. Verify only expected baseline ports are listening.
5. Confirm the source server was not modified and no DNS or application data was changed.
