#[macro_use] extern crate log;
extern crate snapshot_fs;
extern crate env_logger;
extern crate fuse_mt;

fn main() {
    env_logger::init().unwrap();

    let mountpoint = std::env::args_os().nth(1).unwrap();
    let zip_file = std::fs::File::open(std::env::args_os().nth(2).unwrap()).unwrap();
    let fs = snapshot_fs::ArchiveFs::from_zip(zip_file).unwrap();

    debug!("mountpoint: {:?}", mountpoint);

    fuse_mt::mount(fuse_mt::FuseMT::new(fs, 1), &mountpoint, &[]).unwrap();
}
