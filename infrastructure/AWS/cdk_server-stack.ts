import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as iam from "aws-cdk-lib/aws-iam";
import * as s3 from "aws-cdk-lib/aws-s3";

export class StreamingStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // =========================
    // VPC
    // =========================
    const vpc = new ec2.Vpc(this, "Vpc", {
      maxAzs: 2,
      natGateways: 0,
    });

    // =========================
    // Security Group
    // =========================
    const sg = new ec2.SecurityGroup(this, "StreamingSG", {
      vpc,
      allowAllOutbound: true,
    });

    // HTTP / HTTPS — für zukünftigen Reverse Proxy (z. B. Caddy/nginx auf 80/443)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80));
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443));

    // NEU: Operator-UI — nginx Frontend (docker-compose: 3000:80)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3000), "Frontend nginx");

    // NEU: Control Server — Operator-WS + Vehicle-WS + REST (docker-compose: 8080:8080)
    // Vehicle verbindet sich direkt hier (nicht über nginx) für /vehicle/ws
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8080), "Control Server");

    // NEU: MQTT Broker — Fahrzeug-Telemetrie pub/sub (docker-compose: 1883:1883)
    // ACHTUNG: Mosquitto läuft ohne Auth — in Produktion auf Vehicle-IP einschränken
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(1883), "MQTT Broker");

    // GEÄNDERT: coturn STUN/TURN — Host-Port 3479 → Container 3478 (laut docker-compose.yml)
    // War vorher 3478 (falsch für dieses Compose-Mapping)
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3479), "coturn TCP");
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(3479), "coturn UDP");

    // BEHALTEN: coturn TURN Relay-Ports — passt zu min-port=49152 in turnserver.conf
    sg.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.udpRange(49160, 49200),
      "TURN relay",
    );

    // NEU: WebRTC SFU RTP Media — Vehicle → SFU → Operator (docker-compose: 10000-10050:10000-10050/udp)
    sg.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.udpRange(10000, 10050),
      "SFU RTP media",
    );

    // NEU: Grafana Monitoring (docker-compose: 3001:3000)
    // ACHTUNG: In Produktion auf eigene IP einschränken: ec2.Peer.ipv4("DEINE_IP/32")
    sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3001), "Grafana");

    // ENTFERNT:
    // - 8554/tcp (RTSP) — nicht Teil der AVOC-Architektur
    // - 8189/udp (WebRTC alt) — SFU läuft auf 8084 intern, Media über 10000-10050/udp
    // - 8081/tcp "Control WebSocket" — Auth Service ist intern, über nginx auf Port 3000 erreichbar

    // =========================
    // S3 Bucket
    // =========================
    const bucket = new s3.Bucket(this, "AppBucket", {
      versioned: true,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    // =========================
    // EC2
    // =========================
    // HINWEIS: t3.micro (1 GB RAM) ist knapp für 10+ Container.
    // Für stabilen Betrieb: ec2.InstanceSize.SMALL empfohlen.
    // Achtung: Änderung des InstanceType erzwingt CloudFormation-Instance-Replacement.
    const instance = new ec2.Instance(this, "StreamingInstance", {
      vpc,
      instanceType: ec2.InstanceType.of(
        ec2.InstanceClass.T3,
        ec2.InstanceSize.MICRO,
      ),
      machineImage: ec2.MachineImage.latestAmazonLinux2023(),
      vpcSubnets: {
        subnetType: ec2.SubnetType.PUBLIC,
      },
      securityGroup: sg,
    });

    // =========================
    // IAM
    // =========================
    // BEHALTEN: SSM Session Manager für Remote-Zugriff ohne SSH
    instance.role.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName(
        "AmazonSSMManagedInstanceCore",
      ),
    );

    // NEU: SSM Parameter Store — deploy.sh holt Secrets zur Laufzeit aus /avoc/prod/*
    // Kein .env auf der Instanz, kein Docker Secret — nur SSM (ADR-019)
    instance.role.addToPrincipalPolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath",
        ],
        resources: [
          `arn:aws:ssm:${this.region}:${this.account}:parameter/avoc/*`,
        ],
      }),
    );

    // GEÄNDERT: grantRead → grantReadWrite (für zukünftige Audit-Log-Backups, ADR-018)
    bucket.grantReadWrite(instance.role);

    // =========================
    // Elastic IP
    // =========================
    const eip = new ec2.CfnEIP(this, "ElasticIP");

    new ec2.CfnEIPAssociation(this, "EIPAssoc", {
      eip: eip.ref,
      instanceId: instance.instanceId,
    });

    // =========================
    // UserData
    // =========================
    // GEÄNDERT: Docker Compose Plugin v2 (docker compose) statt altem Standalone-Binary
    // 'docker compose' ist der aktuelle Standard — kein separater 'docker-compose' Binary mehr
    // HINWEIS: UserData läuft nur beim ersten Start. Auf bestehender Instanz manuell ausführen:
    //   sudo mkdir -p /usr/local/lib/docker/cli-plugins
    //   sudo curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 \
    //     -o /usr/local/lib/docker/cli-plugins/docker-compose
    //   sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
    instance.addUserData(`
#!/bin/bash
set -euxo pipefail

dnf update -y
dnf install -y docker aws-cli

# Docker Compose Plugin v2 — 'docker compose' (kein Bindestrich)
mkdir -p /usr/local/lib/docker/cli-plugins
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \\
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

systemctl enable docker
systemctl start docker

usermod -aG docker ec2-user || true
usermod -aG docker ssm-user || true

mkdir -p /home/ec2-user/app
chown ec2-user:ec2-user /home/ec2-user/app
`);

    // =========================
    // Outputs
    // =========================
    new cdk.CfnOutput(this, "PublicIP", {
      value: eip.ref,
    });

    new cdk.CfnOutput(this, "BucketName", {
      value: bucket.bucketName,
    });

    new cdk.CfnOutput(this, "InstanceId", {
      value: instance.instanceId,
    });
  }
}
