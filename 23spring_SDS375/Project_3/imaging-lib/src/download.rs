use std::{
    io::{Read, Seek, Write},
    sync::Mutex,
    time::Duration,
};

use futures::stream::StreamExt as _;
#[allow(unused_imports)]
use log::{debug, error, info, warn};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use stream::StreamExt;
use tokio::runtime::Builder as RuntimeBuilder;
use tokio_stream::{self as stream};
use zip::ZipWriter;

use crate::Progress;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ImageDownloadRecord {
    pub filename: String,
    pub url: String,

    #[serde(default)]
    pub completed: bool,
    pub error: Option<String>,
}

pub fn download_images<R: Read + Seek, W1: Write, W2: Write + Seek>(
    csv_input: &mut csv::Reader<R>,
    csv_output: &mut csv::Writer<W1>,
    zip_output: &mut ZipWriter<W2>,
    limit: usize,
) {
    let rt = RuntimeBuilder::new_multi_thread()
        .worker_threads(4)
        .enable_all()
        .build()
        .unwrap();

    // skip the headers
    csv_input.headers().unwrap();
    let csv_input_origin = csv_input.position().to_owned();

    let total_records = csv_input
        .deserialize::<ImageDownloadRecord>()
        .map(|record| record.expect("Failed to parse CSV record."))
        .count();

    let progress = Mutex::new(Progress {
        total: total_records,
        skipped: if limit > 0 && limit < total_records {
            total_records - limit
        } else {
            0
        },
        ..Default::default()
    });
    csv_input.seek(csv_input_origin).unwrap();

    rt.block_on(async {
        let client = Client::default();
        let output_zip = Mutex::new(zip_output);
        let output_csv = Mutex::new(csv_output);

        let input_stream = stream::iter(
            csv_input
                .deserialize::<ImageDownloadRecord>()
                .take(if limit > 0 { limit } else { usize::MAX })
                .map(|record| record.expect("Failed to parse CSV record."))
                .filter(|record| {
                    if record.completed {
                        output_csv
                            .lock()
                            .unwrap()
                            .serialize(record)
                            .expect("Failed to write CSV record.");

                        progress.lock().unwrap().skipped += 1;
                    }

                    !record.completed
                }),
        )
        .throttle(Duration::from_millis(200));

        let responses = futures::StreamExt::map(input_stream, |record| {
            let record_c = record.clone();
            let client_ref = &client;

            async move {
                let resp = client_ref.get(record.url).send().await;
                (
                    match resp {
                        Ok(resp) => match resp.error_for_status_ref() {
                            Ok(_) => {
                                let body = resp.bytes().await;
                                match body {
                                    Ok(body) => Ok(body),
                                    Err(e) => Err(e),
                                }
                            }
                            Err(e) => Err(e),
                        },
                        Err(e) => Err(e),
                    },
                    record_c,
                )
            }
        })
        .buffer_unordered(6);

        responses
            .for_each(|(resp, record)| async {
                let mut record = record;

                match resp {
                    Ok(body) => {
                        record.completed = true;
                        record.error = None;
                        let mut output_zip = output_zip.lock().unwrap();
                        output_zip
                            .start_file(record.filename.clone(), zip::write::FileOptions::default())
                            .unwrap();
                        output_zip.write_all(&body).unwrap();
                    }
                    Err(e) => {
                        record.completed = false;
                        record.error = Some(e.to_string());
                    }
                }

                let mut progress = progress.lock().unwrap();
                progress.completed += if record.completed { 1 } else { 0 };
                progress.failed += if record.completed { 0 } else { 1 };
                info!("Progress: {}", progress);

                output_csv.lock().unwrap().serialize(record).unwrap();
            })
            .await;
    })
}
