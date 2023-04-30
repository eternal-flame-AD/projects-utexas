use std::{
    ffi::{c_char, CString},
    fs::{File, OpenOptions},
    mem::transmute,
    path::Path,
    ptr,
    str::FromStr,
    sync::Mutex,
};

use libR_sys::{
    DllInfo, R_CallMethodDef, R_registerRoutines, R_useDynamicSymbols, Rf_asChar, Rf_mkString,
    Rf_translateCharUTF8, Rf_warning, Rprintf, SEXP,
};
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

#[export_name = "R_init_libimaging_lib"]
#[no_mangle]
pub extern "C" fn R_init(dllinfo: *mut DllInfo) {
    unsafe {
        R_LOGGER.set_max_level(Level::Info);
        log::set_logger(&R_LOGGER).expect("Failed to set logger.");
        log::set_max_level(Level::Info.to_level_filter());
    }

    let call_routines = [
        R_CallMethodDef {
            name: "imaging_lib_init\0".as_ptr() as *const c_char,
            fun: Some(unsafe { transmute(r_img_init as extern "C" fn(RObj) -> SEXP) }),
            numArgs: 1,
        },
        R_CallMethodDef {
            name: "imaging_lib_download\0".as_ptr() as *const c_char,
            fun: Some(unsafe {
                transmute(r_img_download as extern "C" fn(RObj, RObj, RObj, RObj, RObj) -> SEXP)
            }),
            numArgs: 5,
        },
        R_CallMethodDef {
            name: "imaging_lib_extract\0".as_ptr() as *const c_char,
            fun: Some(unsafe {
                transmute(
                    r_img_extract
                        as extern "C" fn(RObj, RObj, RObj, RObj, RObj, RObj, RObj) -> SEXP,
                )
            }),
            numArgs: 7,
        },
        R_CallMethodDef {
            name: ptr::null(),
            fun: None,
            numArgs: 0,
        },
    ];

    unsafe {
        R_registerRoutines(
            dllinfo,
            ptr::null(),
            call_routines.as_ptr(),
            ptr::null(),
            ptr::null(),
        );

        R_useDynamicSymbols(dllinfo, 0);
    }

    info!("libimaging-lib loaded.");
}

#[export_name = "imaging_lib_init"]
#[no_mangle]
pub extern "C" fn r_img_init(log_level: RObj) -> SEXP {
    let log_level: String = log_level.try_into().expect("Failed to convert log level.");
    let log_level = Level::from_str(&log_level).expect("Failed to parse log level.");

    unsafe {
        R_LOGGER.set_max_level(log_level);
        log::set_max_level(log_level.to_level_filter());
    }

    make_r_string("".to_string())
}

#[export_name = "imaging_lib_download"]
#[no_mangle]
pub extern "C" fn r_img_download(
    input_filename: RObj,
    output_filename: RObj,
    output_zip_filename: RObj,
    limit: RObj,
    overwrite: RObj,
) -> SEXP {
    let input_filename: String = input_filename
        .try_into()
        .expect("Failed to convert input filename.");
    let output_filename: String = output_filename
        .try_into()
        .expect("Failed to convert output filename.");
    let output_zip_filename: String = output_zip_filename
        .try_into()
        .expect("Failed to convert output zip filename.");
    let limit: usize = limit.try_into().expect("Failed to convert limit.");
    let overwrite: bool = overwrite.try_into().expect("Failed to convert overwrite.");

    let csv_input_f = File::open(input_filename).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if !overwrite && Path::exists(output_filename.as_ref()) {
        info!("Output CSV file already exists.");
        return make_r_string("Output CSV file already exists.".to_string());
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

    make_r_string("Finished downloading images.".to_string())
}

#[export_name = "imaging_lib_extract"]
#[no_mangle]
pub extern "C" fn r_img_extract(
    input_filename: RObj,
    output_filename: RObj,
    input_zip_filename: RObj,
    num_threads: RObj,
    limit: RObj,
    overwrite: RObj,
    verbose: RObj,
) -> SEXP {
    let input_filename: String = input_filename
        .try_into()
        .expect("Failed to convert input filename.");
    let output_filename: String = output_filename
        .try_into()
        .expect("Failed to convert output filename.");
    let input_zip_filename: String = input_zip_filename
        .try_into()
        .expect("Failed to convert input zip filename.");
    let num_threads = num_threads
        .try_into()
        .expect("Failed to convert num_threads.");
    let limit = limit.try_into().expect("Failed to convert limit.");
    let overwrite: bool = overwrite.try_into().expect("Failed to convert overwrite.");
    let verbose = verbose.try_into().expect("Failed to convert verbose.");

    let csv_input_f = File::open(input_filename).expect("Failed to open input CSV file.");
    let mut csv_input = csv::Reader::from_reader(csv_input_f);

    if !overwrite && Path::exists(output_filename.as_ref()) {
        info!("Output CSV file already exists.");
        return make_r_string("Output CSV file already exists.".to_string());
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

    make_r_string("Finished extracting features".to_string())
}

fn make_r_string(r_string: String) -> SEXP {
    let c_string = std::ffi::CString::new(r_string).unwrap();
    unsafe { Rf_mkString(c_string.as_ptr()) }
}

#[repr(transparent)]
pub struct RObj(SEXP);

impl Into<bool> for RObj {
    fn into(self) -> bool {
        unsafe { libR_sys::Rf_asLogical(self.0) != 0 }
    }
}

impl Into<usize> for RObj {
    fn into(self) -> usize {
        unsafe { libR_sys::Rf_asInteger(self.0) as usize }
    }
}

impl TryInto<String> for RObj {
    type Error = Box<dyn std::error::Error>;

    fn try_into(self) -> Result<String, Self::Error> {
        let r_string = unsafe { Rf_asChar(self.0) };
        let c_string = unsafe { Rf_translateCharUTF8(r_string) };
        let rust_string = unsafe { std::ffi::CStr::from_ptr(c_string).to_str()?.to_owned() };
        Ok(rust_string)
    }
}
