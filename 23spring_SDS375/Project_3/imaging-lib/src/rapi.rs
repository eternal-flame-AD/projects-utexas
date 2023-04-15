use std::{
    ffi::CString,
    fs::{File, OpenOptions},
    path::Path,
    str::FromStr,
    sync::Mutex,
};

use libR_sys::{Rf_asChar, Rf_mkCharCE, Rf_translateCharUTF8, Rf_warning, Rprintf, SEXP};
use log::{info, Level, Log};

use crate::{
    download::{download_images, ImageDownloadRecord},
    features::extract_features,
};

struct RLogger {
    max_level: Level,
}

impl RLogger {
    const fn new() -> Self {
        Self {
            max_level: Level::Info,
        }
    }

    fn set_max_level(&mut self, max_level: Level) {
        self.max_level = max_level;
    }
}

impl Log for RLogger {
    fn enabled(&self, metadata: &log::Metadata) -> bool {
        metadata.level() <= self.max_level
    }

    fn log(&self, record: &log::Record) {
        if self.enabled(record.metadata()) {
            let level = record.level();

            let msg = format!("[{}] [{}] {}", level, record.target(), record.args());
            let msg_c = CString::new(msg).expect("Failed to convert message to CString.");

            unsafe {
                match level {
                    Level::Warn => {
                        let fmtstr = "%s";
                        let fmtstr_c =
                            CString::new(fmtstr).expect("Failed to convert message to CString.");
                        Rf_warning(fmtstr_c.as_ptr(), msg_c.as_ptr())
                    }
                    _ => {
                        let fmtstr = "%s\n";
                        let fmtstr_c =
                            CString::new(fmtstr).expect("Failed to convert message to CString.");
                        Rprintf(fmtstr_c.as_ptr(), msg_c.as_ptr());
                    }
                }
            }
        }
    }

    fn flush(&self) {}
}

static mut R_LOGGER: RLogger = RLogger::new();

#[export_name = "imaging_lib_init"]
#[no_mangle]
pub extern "C" fn r_img_init(log_level: SEXP) -> SEXP {
    let log_level = parse_r_string(log_level).expect("Failed to parse log level.");
    let log_level = Level::from_str(&log_level).expect("Failed to parse log level.");

    unsafe {
        R_LOGGER.set_max_level(log_level);
        log::set_logger(&R_LOGGER).expect("Failed to set logger.");
        log::set_max_level(log_level.to_level_filter());
    }

    make_r_char("".to_string())
}

#[export_name = "imaging_lib_download"]
#[no_mangle]
pub extern "C" fn r_img_download(
    input_filename: SEXP,
    output_filename: SEXP,
    output_zip_filename: SEXP,
    limit: SEXP,
    overwrite: SEXP,
) -> SEXP {
    let input_filename = parse_r_string(input_filename).expect("Failed to parse input filename.");
    let output_filename =
        parse_r_string(output_filename).expect("Failed to parse output filename.");
    let output_zip_filename =
        parse_r_string(output_zip_filename).expect("Failed to parse output zip filename.");
    let limit = parse_r_int(limit).expect("Failed to parse limit.");
    let overwrite = parse_r_bool(overwrite).expect("Failed to parse overwrite.");

    let csv_input_f = File::open(input_filename).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if !overwrite && Path::exists(output_filename.as_ref()) {
        info!("Output CSV file already exists.");
        return make_r_char("Output CSV file already exists.".to_string());
    }

    let csv_output_f = File::create(output_filename).expect("Failed to create output CSV file.");
    let mut csv_output = csv::Writer::from_writer(csv_output_f);

    let mut zip_output;
    if Path::exists(output_zip_filename.as_ref()) {
        let zip_output_f = OpenOptions::new()
            .read(true)
            .write(true)
            .append(true)
            .open(output_zip_filename)
            .expect("Failed to open output ZIP file.");
        zip_output =
            zip::ZipWriter::new_append(zip_output_f).expect("Failed to append to ZIP file.");
    } else {
        let zip_output_f =
            File::create(output_zip_filename).expect("Failed to create output ZIP file.");
        zip_output = zip::ZipWriter::new(zip_output_f);
    }

    info!("Starting image download.");

    download_images(&mut csv_input, &mut csv_output, &mut zip_output, limit);

    make_r_char("Finished downloading images.".to_string())
}

#[export_name = "imaging_lib_extract"]
#[no_mangle]
pub extern "C" fn r_img_extract(
    input_filename: SEXP,
    output_filename: SEXP,
    input_zip_filename: SEXP,
    num_threads: SEXP,
    limit: SEXP,
    overwrite: SEXP,
    verbose: SEXP,
) -> SEXP {
    let input_filename = parse_r_string(input_filename).expect("Failed to parse input filename.");
    let output_filename =
        parse_r_string(output_filename).expect("Failed to parse output filename.");
    let input_zip_filename =
        parse_r_string(input_zip_filename).expect("Failed to parse input zip filename.");
    let num_threads = parse_r_int(num_threads).expect("Failed to parse num_images.");
    let limit = parse_r_int(limit).expect("Failed to parse limit.");
    let overwrite = parse_r_bool(overwrite).expect("Failed to parse overwrite.");
    let verbose = parse_r_bool(verbose).expect("Failed to parse verbose.");

    let csv_input_f = File::open(input_filename).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if !overwrite && Path::exists(output_filename.as_ref()) {
        info!("Output CSV file already exists.");
        return make_r_char("Output CSV file already exists.".to_string());
    }

    let csv_output_f = File::create(output_filename).expect("Failed to create output CSV file.");
    let mut csv_output = csv::Writer::from_writer(csv_output_f);

    let zip_input_f = File::open(input_zip_filename).expect("Failed to open input ZIP file.");
    let zip_input = zip::ZipArchive::new(zip_input_f).expect("Failed to open ZIP file.");

    info!("Starting feature extraction.");

    extract_features(
        csv_input
            .deserialize()
            .map(|r| r.expect("Failed to deserialize CSV record."))
            .filter(|r: &ImageDownloadRecord| r.completed),
        &mut csv_output,
        Mutex::new(zip_input),
        num_threads,
        limit,
        verbose,
    );

    make_r_char("Finished extracting features".to_string())
}

fn make_r_char(r_string: String) -> SEXP {
    let c_string = std::ffi::CString::new(r_string).unwrap();
    unsafe { Rf_mkCharCE(c_string.as_ptr(), libR_sys::cetype_t_CE_UTF8) }
}

fn parse_r_string(r_string: SEXP) -> Result<String, Box<dyn std::error::Error>> {
    let r_string = unsafe { Rf_asChar(r_string) };
    let c_string = unsafe { Rf_translateCharUTF8(r_string) };
    let rust_string = unsafe { std::ffi::CStr::from_ptr(c_string).to_str()?.to_owned() };
    Ok(rust_string)
}

fn parse_r_int(r_int: SEXP) -> Result<usize, Box<dyn std::error::Error>> {
    let rust_int = unsafe { libR_sys::Rf_asInteger(r_int) };
    Ok(rust_int as usize)
}

fn parse_r_bool(r_bool: SEXP) -> Result<bool, Box<dyn std::error::Error>> {
    let rust_bool = unsafe { libR_sys::Rf_asLogical(r_bool) };
    Ok(rust_bool != 0)
}
