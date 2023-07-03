# Telegram Translation Bot


## 1. Prerequisites

* Install the latest version of Easegress according to [this document](https://github.com/megaease/easegress#setting-up-easegress) and make sure that external applications can access the Easegress instance on at least one of ports 80, 88, 443 or 8443.
* Create a Telegram bot by following [this document](https://core.telegram.org/bots#3-how-do-i-create-a-bot), set its name, write down its token, and [set up a Webhook](https://core.telegram.org/bots/api#setwebhook) that points to the Easegress instance installed in the previous step. The bot will receive notifications of new messages through this Webhook. Note the webhook url must be an `https` one.
* AWS Access Key ID and Access Key Secret, and ensure that you can use the AWS translation API with this Access Key.
* Google Cloud's Token and ensure that you can use Google Cloud's Speech Recognize API and OCR (Image Annotation) API with this Token.

## 2. Update The Configuration Files

The default content of the example configuration may not work for you, please update it according to your requirement, please check the comments in the example configuration files to find out what you should change.

## 3. Deploy

* Deploy the AutoCertManager, it can apply and renew the SSL certificates from [Let's Encrypt](https://letsencrypt.org/) automatically. This step is optional, it is only needed if you want Easegress to manage the SSL certificates.

```bash
$ egctl create -f auto-cert.yaml
```

* Deploy the HTTPServer.

```bash
$ egctl create -f http-server.yaml
```

* Deploy the Pipeline.

```bash
$ egctl create -f translate-pipeline.yaml
```

There should be no output after executing the commands, and if there's any, it means there are grammar errors in the configuration, please fix them and try again.

After all configuration have been deployed, please use below command to check if everything is correct:

```bash
$ curl https://{YOUR DOMAIN NAME}:{YOUR PORT}/translate -i
```

The status code should be `200`.

## 4. Trouble Shooting

If the configuration were deployed successfully but the bot is not working as expect, we can use the `log` function in the `template` of the builder filters for help:

```yaml
- kind: RequestBuilder
  name: requestBuilderSpeechText
  template: |
    {{log "warn" .responses.extract.Body}}                         # +
    {{$result := index .responses.extract.JSONBody.results 0}}
    {{$result = index $result.alternatives 0}}
    body: |
      {"text": "{{$result.transcript | jsonEscape}}"}
```

For example, the `log` function above will print out the response body of the speech recognize API.

Please make sure to apply the changes to Easegress after you modified one of the configuration files with below command:

```bash
$ egctl apply -f {file name}.yaml
```

Please check [Build A Telegram Translation Bot With Easegress](../../doc/cookbook/translation-bot.md)
for more information about the bot.