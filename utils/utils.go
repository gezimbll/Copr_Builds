package utils

import "github.com/google/uuid"

const (
	FedoraBroker = "amqps://fedora:@rabbitmq.fedoraproject.org/%2Fpublic_pubsub"
	Exchange     = "amq.topic"
	RoutingKey   = "org.fedoraproject.prod.copr.build.end"
	Owner        = "gzim07"

	//cacert and key paths
	CaCert = "/etc/fedora-messaging/cacert.pem"
	Cert   = "/etc/fedora-messaging/fedora-cert.pem"
	Key    = "/etc/fedora-messaging/fedora-key.pem"

	DownloadUrl = "https://download.copr.fedorainfracloud.org/results/"
	Prefix      = "cgrates-"
	RpmSuffix   = "rpm"
	ArchBuild   = "x86_64"
	Current     = "cgrates-current"
	PackageDir  = "/var/packages/rpm"
)

func NewUuid() string {
	queueUUID := uuid.New()
	return queueUUID.String()
}
