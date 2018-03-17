# rninja
Micro reverse proxy that takes care of TLS certificates generation and renewal via Let's Encrypt.
This is something that you can run on a same host with your main web application that does not speak TLS.

Installation:
`go get -u github.com/cooldarkdryplace/rninja`

To install as a systemd service copy `rninja.service` file to your `/etc/systemd/services/` folder. Update `Environment` variables.

* Enable it: `systemctl enable rninja`
* Start it: `systemctl start rninja`
* Check it: `systemctl status rninja`

Enjoy.
