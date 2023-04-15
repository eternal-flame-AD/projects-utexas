use std::{
    cell::RefCell,
    fs::File,
    io::{Cursor, Read, Write},
    ops::Deref,
    rc::Rc,
    sync::{atomic::AtomicBool, Mutex},
};

use image::io::Reader as ImageReader;
use image::{ImageBuffer, Pixel, Primitive, Rgb};
use itertools::Itertools;
use kmeans_colors::{get_kmeans, Calculate, Kmeans, Sort};
#[allow(unused_imports)]
use log::{debug, error, info, warn};
use palette::{IntoColor, Lab, Srgb};
use rayon::ThreadPoolBuilder;

use crate::download::ImageDownloadRecord;
use crate::Progress;

/// Extract features in parallel from iterator of image records.
/// Write extracted features to CSV.
pub fn extract_features<I: Iterator<Item = ImageDownloadRecord> + Send, W: Write + Send>(
    input: I,
    output: &mut csv::Writer<W>,
    image_zip: Mutex<zip::ZipArchive<File>>,
    nthreads: usize,
    limit: usize,
    verbose: bool,
) {
    let pool = ThreadPoolBuilder::default()
        .num_threads(nthreads)
        .build()
        .expect("Failed to build thread pool.");

    // thread-local variables
    thread_local! {
        static EXTRACTORS: RefCell<Option<[Box<dyn RGBFeatureExtractor<u8>>; 3]>> = RefCell::new(None);
    }

    // shared variables
    let output = Mutex::new(output);
    let header_written = AtomicBool::new(false);
    let progress = Mutex::new(Progress::default());

    // initialize thread-local
    pool.broadcast(|_| {
        EXTRACTORS.with(|extractors| {
            *extractors.borrow_mut() = Some([
                Box::new(BrightnessAndContrastFeatureExtractor {}),
                Box::new(DimensionFeatureExtractor {}),
                Box::new(ColorClusterFeatureExtractor {
                    n_clusters_out: 3,
                    n_clusters_max: 10,
                    n_runs: 8,
                    verbose,
                }),
            ]);
        });
    });

    // start pipeline
    pool.install(|| {
        rayon::scope(|s| {
            for rec in input.take(if limit == 0 { usize::MAX } else { limit }) {
                let rec = rec.clone();

                s.spawn(|_| {
                    let mut image_buf = Vec::with_capacity(1024 * 128);

                    // read image data from zip
                    {
                        let mut zip = image_zip.lock().unwrap();
                        let mut zipfile = zip
                            .by_name(&rec.filename)
                            .expect("Failed to open file in zip.");

                        zipfile
                            .read_to_end(&mut image_buf)
                            .expect("Failed to read image from ZIP file.");
                    }

                    let image = ImageReader::new(Cursor::new(image_buf))
                        .with_guessed_format()
                        .expect("Failed to guess image format.")
                        .decode();

                    if verbose {
                        info!("Extracting features for {}", rec.filename);
                    }

                    // extract features
                    let result = match image {
                        Ok(image) => {
                            let image = image.to_rgb8();

                            let features = EXTRACTORS.with(|extractors| {
                                extractors
                                    .borrow()
                                    .as_ref()
                                    .unwrap()
                                    .iter()
                                    .map(|extractor| extractor.extract(&image))
                                    .collect::<Vec<_>>()
                            });

                            Some((rec, features))
                        }
                        Err(e) => {
                            eprintln!("Failed to decode image: {}", e);
                            None
                        }
                    };

                    // write output
                    let mut progress = progress.lock().unwrap();
                    if let Some((rec, features)) = result {
                        let mut output = output.lock().unwrap();

                        if !header_written.load(std::sync::atomic::Ordering::Relaxed) {
                            let mut header = vec!["url".to_string(), "filename".to_string()];
                            for feature in &features {
                                header.extend(feature.headers());
                            }
                            output
                                .write_record(header)
                                .expect("Failed to write CSV header.");
                            header_written.store(true, std::sync::atomic::Ordering::Relaxed);
                        }

                        let mut record = vec![rec.url, rec.filename];
                        for feature in features {
                            record.extend(feature.values());
                        }

                        output
                            .write_record(record)
                            .expect("Failed to write CSV record.");

                        if verbose {
                            output.flush().expect("Failed to flush CSV output.");
                        }

                        progress.add_completed();
                    } else {
                        progress.add_failed();
                    }

                    info!("Progress: {}", progress);
                })
            }
        });
    });
}

/// Abstraction for feature extractors.
pub trait RGBFeatureExtractor<T>
where
    Rgb<T>: Pixel,
{
    fn extract(&self, image: &ImageBuffer<Rgb<T>, Vec<T>>) -> Box<dyn CsvRecordProvider + Send>;
}

/// Abstraction for features that can be written to CSV.
pub trait CsvRecordProvider {
    fn headers(&self) -> Vec<String>;
    fn values(&self) -> Vec<String>;
}

pub struct DimensionFeature {
    pub width: u32,
    pub height: u32,

    pub asp_ratio: f64,
}

/// Extracts image dimensions.
pub struct DimensionFeatureExtractor;

impl<T> RGBFeatureExtractor<T> for DimensionFeatureExtractor
where
    Rgb<T>: Pixel,
    T: Primitive,
    Vec<T>: Deref<Target = [<Rgb<T> as Pixel>::Subpixel]>,
{
    fn extract(&self, image: &ImageBuffer<Rgb<T>, Vec<T>>) -> Box<dyn CsvRecordProvider + Send> {
        let (width, height) = image.dimensions();
        let asp_ratio = width as f64 / height as f64;

        Box::new(DimensionFeature {
            width,
            height,
            asp_ratio,
        })
    }
}

impl CsvRecordProvider for DimensionFeature {
    fn headers(&self) -> Vec<String> {
        vec![
            "width".to_string(),
            "height".to_string(),
            "asp_ratio".to_string(),
        ]
    }

    fn values(&self) -> Vec<String> {
        vec![
            self.width.to_string(),
            self.height.to_string(),
            self.asp_ratio.to_string(),
        ]
    }
}

pub struct BrightnessAndContrastFeature {
    pub brightness: [f64; 4],
    pub contrast: [f64; 4],
}

impl CsvRecordProvider for BrightnessAndContrastFeature {
    fn headers(&self) -> Vec<String> {
        vec![
            "brightness_r".to_string(),
            "brightness_g".to_string(),
            "brightness_b".to_string(),
            "brightness_avg".to_string(),
            "contrast_r".to_string(),
            "contrast_g".to_string(),
            "contrast_b".to_string(),
            "contrast_avg".to_string(),
        ]
    }

    fn values(&self) -> Vec<String> {
        vec![
            self.brightness[0].to_string(),
            self.brightness[1].to_string(),
            self.brightness[2].to_string(),
            self.brightness[3].to_string(),
            self.contrast[0].to_string(),
            self.contrast[1].to_string(),
            self.contrast[2].to_string(),
            self.contrast[3].to_string(),
        ]
    }
}

/// Extracts brightness and contrast by RGB channel.
pub struct BrightnessAndContrastFeatureExtractor;

impl RGBFeatureExtractor<u8> for BrightnessAndContrastFeatureExtractor {
    fn extract(&self, image: &ImageBuffer<Rgb<u8>, Vec<u8>>) -> Box<dyn CsvRecordProvider + Send> {
        let (width, height) = image.dimensions();

        let mut feature = BrightnessAndContrastFeature {
            brightness: [0.0; 4],
            contrast: [0.0; 4],
        };

        for channel in [0, 1, 2, 3] {
            let brightness = image
                .pixels()
                .map(|pixel| {
                    let val: u64 = match channel {
                        0 => pixel[0] as u64,
                        1 => pixel[1] as u64,
                        2 => pixel[2] as u64,
                        3 => (pixel[0] as u64 + pixel[1] as u64 + pixel[2] as u64) / 3,
                        _ => unreachable!(),
                    };

                    val
                })
                .sum::<u64>()
                / (width * height) as u64;

            feature.brightness[channel] = brightness as f64 / 255.0;

            let contrast = (image
                .pixels()
                .map(|pixel| {
                    let val: f64 = match channel {
                        0 => pixel[0] as f64,
                        1 => pixel[1] as f64,
                        2 => pixel[2] as f64,
                        3 => (pixel[0] as f64 + pixel[1] as f64 + pixel[2] as f64) / 3.0,
                        _ => unreachable!(),
                    };

                    (val - brightness as f64).powi(2)
                })
                .sum::<f64>()
                / (width * height) as f64)
                .sqrt();

            feature.contrast[channel] = contrast / 255.0;
        }

        Box::new(feature)
    }
}

pub struct ColorClusterFeature {
    pub cluster: Vec<Lab>,
    pub k: usize,
}

/// Extracts dominant colors using k-means clustering.
pub struct ColorClusterFeatureExtractor {
    pub n_clusters_out: usize,
    pub n_clusters_max: usize,
    pub n_runs: usize,
    pub verbose: bool,
}

impl CsvRecordProvider for ColorClusterFeature {
    fn headers(&self) -> Vec<String> {
        let mut headers = vec!["color_cluster_k".to_string()];
        for i in 0..self.cluster.len() {
            for c in ["l", "a", "b"].iter() {
                headers.push(format!("color_cluster_{}_{}", i, c));
            }
        }
        headers
    }

    fn values(&self) -> Vec<String> {
        vec![self.k.to_string()]
            .into_iter()
            .chain(
                self.cluster
                    .iter()
                    .flat_map(|c| [c.l, c.a, c.b])
                    .map(|v| v.to_string()),
            )
            .collect()
    }
}

/// compute total within sum of squares
fn kmeans_withinss<K: Calculate>(km: Kmeans<K>, data: &[K]) -> f64 {
    let n_centroids = km.centroids.len();

    let mut withinss = vec![0.0f64; n_centroids];

    for (label, point) in km.indices.iter().zip(data.iter()) {
        let cluster_i = *label as usize;
        let centroid = &km.centroids[cluster_i];
        withinss[cluster_i] += Calculate::difference(centroid, point) as f64;
    }

    withinss.iter().sum()
}

impl RGBFeatureExtractor<u8> for ColorClusterFeatureExtractor {
    fn extract(&self, image: &ImageBuffer<Rgb<u8>, Vec<u8>>) -> Box<dyn CsvRecordProvider + Send> {
        // convert to Lab colorpace
        let lab = image
            .pixels()
            .map(|p| Srgb::new(p[0], p[1], p[2]).into_format().into_color())
            .collect::<Vec<Lab>>();

        let (result, _) = (self.n_clusters_out..=(self.n_clusters_max + 1))
            // try n_clusters from n_clusters_out to n_clusters_max
            .map(|n_clusters| {
                // run multiple times and pick the best result
                let result = (0..self.n_runs)
                    .map(|i| {
                        let run_result = get_kmeans(
                            n_clusters, // n_clusters
                            50,         // n_iterations
                            0.0005,     // tolerance
                            false, &lab, i as u64, // seed
                        );

                        run_result
                    })
                    .min_by(|a, b| a.score.partial_cmp(&b.score).unwrap())
                    .unwrap();
                let tot_withinss = kmeans_withinss(result.clone(), &lab);

                // reduce cloning
                Rc::new((result, tot_withinss))
            })
            .tuple_windows()
            // stop when the withinss ratio to next cluster size is less than 0.7, return result
            .find_map(|(result, next_result)| {
                let n_clusters = result.0.centroids.len();
                let withinss = result.1;
                let withinss_ratio = next_result.1 / withinss;

                if self.verbose {
                    info!(
                        "n_clusters = {}, tot_withinss = {}, ratio = {}",
                        n_clusters, withinss, withinss_ratio
                    );
                }

                if withinss_ratio > 0.7 || n_clusters == self.n_clusters_max {
                    Some((result.0.clone(), withinss))
                } else {
                    None
                }
            })
            .unwrap();

        // sort clusters by percentage
        let mut pal = Lab::sort_indexed_colors(&result.centroids, &result.indices);
        pal.sort_unstable_by(|a, b| (b.percentage).partial_cmp(&a.percentage).unwrap());

        // collect into vec of colors
        let clusters = pal
            .iter()
            .take(self.n_clusters_out)
            .map(|c| c.centroid.into_color())
            .collect::<Vec<Lab>>();

        Box::new(ColorClusterFeature {
            cluster: clusters,
            k: result.centroids.len(),
        })
    }
}
