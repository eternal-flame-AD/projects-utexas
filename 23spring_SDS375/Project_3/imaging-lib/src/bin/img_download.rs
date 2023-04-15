use std::{
    fs::{File, OpenOptions},
    path::Path,
};

use clap::Parser;
use imaging_lib::download::download_images;
#[allow(unused_imports)]
use log::{debug, error, info, warn};

#[derive(Parser, Debug)]
#[command(name = "img-download")]
struct Args {
    #[clap(short, long, default_value = "data/horror_movies_urls_in.csv")]
    input: String,

    #[clap(short, long, default_value = "data/horror_movies_urls_out.csv")]
    output: String,

    #[clap(short, long, default_value = "data/images.zip")]
    zip_path: String,

    #[clap(long, default_value = "0")]
    limit: usize,
}

fn main() {
    simple_logger::init_with_env().unwrap();
    let args = Args::parse();

    debug!("Parsed arguments: {:?}", args);

    let csv_input_f = File::open(args.input).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if Path::exists(args.output.as_ref()) {
        panic!("Output CSV file already exists.");
    }

    let csv_output_f = File::create(args.output).expect("Failed to create output CSV file.");
    let mut csv_output = csv::Writer::from_writer(csv_output_f);

    let mut zip_output;
    if Path::exists(args.zip_path.as_ref()) {
        let zip_output_f = OpenOptions::new()
            .read(true)
            .write(true)
            .append(true)
            .open(args.zip_path)
            .expect("Failed to open output ZIP file.");
        zip_output =
            zip::ZipWriter::new_append(zip_output_f).expect("Failed to append to ZIP file.");
    } else {
        let zip_output_f = File::create(args.zip_path).expect("Failed to create output ZIP file.");
        zip_output = zip::ZipWriter::new(zip_output_f);
    }

    info!("Starting image download.");

    download_images(&mut csv_input, &mut csv_output, &mut zip_output, args.limit);
}
