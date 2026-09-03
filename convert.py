import sys
import polars as pl
import os

csv, out = sys.argv[1], sys.argv[2]

with open(csv) as f:
    first = f.readline()
HAS_HEADER = first.split(",")[0] == "id"

def process_batch(df):
    df = df.select([pl.col(df.columns[1]), pl.col(df.columns[2]),
                    pl.col(df.columns[3]), pl.col(df.columns[4])])
    df.columns = ["price", "qty", "quote_qty", "timestamp"]
    # Binance trades timestamp: 毫秒(13位, <=2024) 或 微秒(16位, >=2025)
    DIV = pl.when(pl.col("timestamp") > 10**15).then(1_000_000).otherwise(1_000)
    return (
        df.with_columns((pl.col("timestamp") // DIV).alias("ts_sec"))
        .sort("timestamp")
        .group_by("ts_sec", maintain_order=True)
        .agg([
            pl.col("price").first().alias("open"),
            pl.col("price").max().alias("high"),
            pl.col("price").min().alias("low"),
            pl.col("price").last().alias("close"),
            pl.col("qty").sum().alias("volume"),
            pl.col("quote_qty").sum().alias("quote_volume"),
            pl.len().alias("trade_count"),
            pl.col("timestamp").first().alias("first_trade_ms"),
            pl.col("timestamp").last().alias("last_trade_ms"),
        ])
    )

reader = pl.read_csv_batched(
    csv, has_header=HAS_HEADER, batch_size=2_000_000,
    schema_overrides={"price": pl.Float64, "qty": pl.Float64,
                      "quote_qty": pl.Float64, "timestamp": pl.Int64},
)
chunks = []
while True:
    b = reader.next_batches(5)
    if b is None or len(b) == 0:
        break
    for df in b:
        chunks.append(process_batch(df))

big = pl.concat(chunks)
final = (
    big.sort("ts_sec")
    .group_by("ts_sec", maintain_order=True)
    .agg([
        pl.col("open").first(), pl.col("high").max(), pl.col("low").min(),
        pl.col("close").last(), pl.col("volume").sum(),
        pl.col("quote_volume").sum(), pl.col("trade_count").sum(),
        pl.col("first_trade_ms").first(), pl.col("last_trade_ms").last(),
    ])
)
print(f"1s bars: {len(final)}")

final = final.with_columns(
    pl.from_epoch(pl.col("ts_sec"), time_unit="s").cast(pl.String).alias("datetime_utc")
)
final.select(["ts_sec", "datetime_utc", "open", "high", "low", "close",
              "volume", "quote_volume", "trade_count",
              "first_trade_ms", "last_trade_ms"]).write_parquet(out)
print(f"saved {out}: {os.path.getsize(out)/1024/1024:.1f} MB")
