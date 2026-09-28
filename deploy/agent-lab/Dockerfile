ARG BASE_IMAGE=ubuntu:24.04
FROM ${BASE_IMAGE}
ENV container=docker
RUN if command -v apt-get >/dev/null; then \
      apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends systemd systemd-sysv openssh-server sudo curl ca-certificates passwd tar procps iproute2 python3 && rm -rf /var/lib/apt/lists/*; \
    else \
      dnf install -y systemd openssh-server sudo curl-minimal ca-certificates shadow-utils tar procps-ng iproute python3 && dnf clean all; \
    fi
RUN useradd -m -s /bin/sh labadmin && \
    printf 'labadmin ALL=(root) NOPASSWD: ALL\n' > /etc/sudoers.d/xingdu-lab && chmod 440 /etc/sudoers.d/xingdu-lab && \
    mkdir -p /home/labadmin/.ssh && chmod 700 /home/labadmin/.ssh && chown -R labadmin:labadmin /home/labadmin/.ssh && \
    rm -f /etc/ssh/ssh_host_* && \
    systemctl mask --force systemd-remount-fs.service getty.target console-getty.service ssh.service ssh.socket sshd.service sshd.socket
COPY deploy/agent-lab/sshd_config /etc/ssh/sshd_config
COPY deploy/agent-lab/sshd.service /etc/systemd/system/xingdu-lab-sshd.service
RUN systemctl enable xingdu-lab-sshd.service
COPY deploy/agent-lab/entrypoint.sh /usr/local/bin/lab-entrypoint
STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/usr/local/bin/lab-entrypoint"]
