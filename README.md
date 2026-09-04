# mackerel-plugin-httping

HTTP/HTTPS のレイテンシを計測する Mackerel カスタムプラグインです。

mackerel-plugin-httping は以下の処理を行います。

- レイテンシ計測の前にプリフライトリクエストを送信し、Keep-Alive と DNS キャッシュを準備します
- 指定した URL に対して順次リクエストを送信します
- 応答の最初の 1 バイトを受信するまでの時間をレイテンシとして計測します
- min / max / 90 パーセンタイル / average を算出します

## インストール方法

リリースページからバイナリをダウンロードするか、`mkr` コマンドを利用してインストールできます。

```sh
mkr plugin install monitoring-forge/mackerel-plugin-httping
```

`mkr` の詳細については [mkr コマンドのドキュメント](https://mackerel.io/ja/docs/entry/advanced/cli) を参照してください。

## 簡単な使い方

```sh
mackerel-plugin-httping --url https://example.com/ --key-prefix example
```

### 出力例

```
httping.example_rtt_count.success       10.000000       1581488858
httping.example_rtt_count.error         0.000000        1581488858
httping.example_rtt_ms.max              111.961772      1581488858
httping.example_rtt_ms.min              108.004440      1581488858
httping.example_rtt_ms.average          109.899642      1581488858
httping.example_rtt_ms.90_percentile    110.906144      1581488858
```

### オプション

```
% ./mackerel-plugin-httping -h
Usage:
  mackerel-plugin-httping [OPTIONS]

Application Options:
      --url=               URL to ping
      --timeout=           timeout millisec per ping (default: 5000)
      --interval=          sleep millisec after every ping (default: 200)
      --count=             Count Sending ping (default: 10)
      --key-prefix=        Metric key prefix
      --disable-keepalive  disable keepalive
  -v, --version            Show version

Help Options:
  -h, --help               Show this help message
```

## mackerel-agent の設定方法

`/etc/mackerel-agent/mackerel-agent.conf` に plugin 設定を追加します。

```conf
[plugin.metrics.httping]
command = ["/usr/local/bin/mackerel-plugin-httping", "--url", "https://example.com/", "--key-prefix", "example", "--count", "10", "--interval", "200"]
```

設定後、`mackerel-agent` を再起動してください。

```sh
sudo systemctl restart mackerel-agent
```

Mackerel 上では `custom.httping.example_rtt_ms.*` などのメトリクス名でグラフが作成されます。

## 出力されるメトリクスの意味

| メトリクス名 | 単位 | 説明 |
| --- | --- | --- |
| `httping.<prefix>_rtt_count.success` | count | 成功したリクエスト数 |
| `httping.<prefix>_rtt_count.error` | count | 失敗したリクエスト数 |
| `httping.<prefix>_rtt_ms.max` | ms | 成功したリクエストのレイテンシの最大値 |
| `httping.<prefix>_rtt_ms.min` | ms | 成功したリクエストのレイテンシの最小値 |
| `httping.<prefix>_rtt_ms.average` | ms | 成功したリクエストのレイテンシの平均値 |
| `httping.<prefix>_rtt_ms.90_percentile` | ms | 成功したリクエストのレイテンシの 90 パーセンタイル |

なお、`rtt_ms.*` の各メトリクスは成功したリクエストのみを対象に算出されます。すべてのリクエストが失敗した場合は `rtt_ms.*` のメトリクスは出力されません。

## License

MIT License
