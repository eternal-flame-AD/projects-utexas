use std::{fs::File, path::Path, sync::Mutex, thread::available_parallelism};

use clap::Parser;
use imaging_lib::{download::ImageDownloadRecord, features::extract_features};
#[allow(unused_imports)]
use log::{debug, error, info, warn};

#[derive(Parser, Debug)]
#[command(name = "img-features")]
struct Args {
    #[clap(short, long, default_value = "data/horror_movies_urls_out.csv")]
    input: String,

    #[clap(short, long, default_value = "data/horror_movies_image_features.csv")]
    output: String,

    #[clap(short, long, default_value = "data/images.zip")]
    zip_path: String,

    #[clap(long, default_value_t = available_parallelism().map(|p| p.get()).unwrap_or(1))]
    num_threads: usize,

    #[clap(long, default_value = "0")]
    limit: usize,

    #[clap(long, default_value = "false")]
    overwrite: bool,

    #[clap(short, long, default_value = "false")]
    verbose: bool,
}

fn main() {
    simple_logger::init_with_env().unwrap();
    let args = Args::parse();

    debug!("Parsed arguments: {:?}", args);

    let csv_input_f = File::open(args.input).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if !args.overwrite && Path::exists(args.output.as_ref()) {
        panic!("Output CSV file already exists.");
    }

    let csv_output_f = File::create(args.output).expect("Failed to create output CSV file.");
    let mut csv_output = csv::Writer::from_writer(csv_output_f);

    let zip_input_f = File::open(args.zip_path).expect("Failed to open input ZIP file.");
    let zip_input = zip::ZipArchive::new(zip_input_f).expect("Failed to open ZIP file.");

    extract_features(
        csv_input
            .deserialize::<ImageDownloadRecord>()
            .map(|r| r.expect("Failed to parse CSV record."))
            .filter(|r| r.completed),
        &mut csv_output,
        Mutex::new(zip_input),
        args.num_threads,
        args.limit,
        args.verbose,
    );
}
