use std::fmt::Display;

pub mod download;

pub mod features;

#[cfg(not(feature = "bin"))]
mod rapi;

#[derive(Default, Debug)]
pub struct Progress {
    completed: usize,
    skipped: usize,
    failed: usize,
    total: usize,
}

impl Display for Progress {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if self.total > 0 {
            let progress =
                (self.completed + self.failed) as f64 / (self.total - self.skipped) as f64;
            write!(
                f,
                "{} of {} completed. ({} skipped, {} failed) ({:.2}%)",
                self.completed,
                self.total - self.skipped - self.failed,
                self.skipped,
                self.failed,
                progress * 100.0
            )
        } else {
            write!(
                f,
                "{} completed. ({} skipped, {} failed)",
                self.completed, self.skipped, self.failed,
            )
        }
    }
}

impl Progress {
    pub fn add_skipped(&mut self) {
        self.skipped += 1;
    }

    pub fn add_completed(&mut self) {
        self.completed += 1;
    }

    pub fn add_failed(&mut self) {
        self.failed += 1;
    }
}
