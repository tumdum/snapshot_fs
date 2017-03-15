#[macro_use]
extern crate log;
extern crate fuse_mt;
extern crate zip;
extern crate libc;
extern crate time;

use fuse_mt::*;
use std::path::{Path, PathBuf};
use std::io::{Read, Seek};
use std::sync::Mutex;
use std::collections::HashSet;
use std::ffi::OsStr;
use time::Timespec;

#[derive(Debug)]
pub enum SnapshotError {
    DummyError,
}

type SnapshotResult<T> = Result<T, SnapshotError>;

pub struct ArchiveFs<R: Read + Seek> {
    z: Mutex<zip::ZipArchive<R>>,
}

impl<R: Read + Seek> ArchiveFs<R> {
    pub fn from_zip(input: R) -> SnapshotResult<ArchiveFs<R>> {
        match zip::ZipArchive::new(input) {
            Ok(z) => Ok(ArchiveFs { z: Mutex::new(z) }),
            Err(_) => Err(SnapshotError::DummyError),
        }
    }

    fn to_zip_path(&self, path: &Path) -> String {
        let p = path.to_str();
        if p.is_none() {
            panic!("TODO: failed to convert path to str");
        }
        let mut p: &str = &p.unwrap().replace("/", "\\");
        if p.starts_with("\\") {
            p = &p[1..];
        }
        p.into()
    }

    fn is_directory(&self, path: &Path) -> bool {
        self.get_file(&mut *self.z.lock().unwrap(), path).is_err()
    }

    fn get_size(&self, path: &Path) -> u64 {
        self.get_file(&mut *self.z.lock().unwrap(), path).unwrap().size()
    }

    fn get_file<'a, X: Read + Seek>(&self, z: &'a mut zip::ZipArchive<X>, path: &Path) -> zip::result::ZipResult<zip::read::ZipFile<'a>> {
        z.by_name(&self.to_zip_path(&path))
    }
}

fn convert_name(name: &str, path: &Path) -> Option<DirectoryEntry> {
    let name_path = PathBuf::from("/").join(name.replace("\\", "/"));
    let suffix = match name_path.strip_prefix(path) {
        Ok(s) => s,
        Err(_) => return None,
    };
    let final_name = suffix.components().next();
    if final_name.is_none() {
        return None;
    }
    let final_name = final_name.unwrap();
    Some(DirectoryEntry {
             name: final_name.as_os_str().into(),
             kind: if suffix.components().count() > 1 {
                 FileType::Directory
             } else {
                 FileType::RegularFile
             },
         })
}

impl<R: Read + Seek> FilesystemMT for ArchiveFs<R> {
    fn init(&self, _req: RequestInfo) -> ResultEmpty {
        debug!("Init called");
        Ok(())
    }

    fn getattr(&self, _req: RequestInfo, path: &Path, _fh: Option<u64>) -> ResultGetattr {
        if self.is_directory(&path) {
            Ok((TTL, HELLO_DIR_ATTR))
        } else {
            let mut attr = HELLO_FILE_ATTR.clone();
            attr.size = self.get_size(path);
            Ok((TTL, attr))
        }
    }

    fn opendir(&self, _req: RequestInfo, _path: &Path, _flags: u32) -> ResultOpen {
        debug!("opendir for '{:?}'", _path);
        Ok((0, 0))
    }

    fn releasedir(&self, _req: RequestInfo, _path: &Path, _fh: u64, _flags: u32) -> ResultEmpty {
        debug!("releasedir for {:?}", _path);
        Ok(())
    }

    fn lookup(&self, _req: RequestInfo, _parent: &Path, _name: &OsStr) -> ResultEntry {
        debug!("lookup for {:?} / {:?}", _parent, _name);
        self.getattr(_req, &_parent.join(_name), None)
    }

    fn readdir(&self, _req: RequestInfo, path: &Path, _fh: u64) -> ResultReaddir {
        debug!("readdir for {:?}", path);
        let mut seen = HashSet::new();
        let mut z = self.z.lock().unwrap();
        let entries = (0..z.len())
            .map(|i| {
                     z.by_index(i)
                         .unwrap()
                         .name()
                         .to_string()
                 })
            .filter_map(|name| convert_name(&name, path))
            .filter(|ref e| if seen.contains(&e.name) {
                        return false;
                    } else {
                        seen.insert(e.name.clone());
                        debug!("entry under {:?}: {:?}", path, e.name);
                        return true;
                    });
        Ok(entries.collect())
    }

    fn open(&self, _req: RequestInfo, _path: &Path, _flags: u32) -> ResultOpen {
        debug!("open for {:?}", _path);
        Ok((0, 0))
    }

    fn read(&self,
            _req: RequestInfo,
            _path: &Path,
            _fh: u64,
            _offset: u64,
            _size: u32)
            -> ResultData {
        debug!("read for {:?}, offset: {:?}, size: {:?}",
               _path,
               _offset,
               _size);
        let l = &mut *self.z.lock().unwrap();
        Ok(self
           .get_file(l, _path)
           .unwrap()
           .bytes()
           .skip(_offset as usize)
           .take(_size as usize)
           .collect::<std::io::Result<Vec<u8>>>()
           .unwrap())
    }
}

const TTL: Timespec = Timespec { sec: 60, nsec: 0 };

const CREATE_TIME: Timespec = Timespec {
    sec: 1381237736,
    nsec: 0,
};

const HELLO_DIR_ATTR: FileAttr = FileAttr {
    ino: 1,
    size: 0,
    blocks: 0,
    atime: CREATE_TIME,
    mtime: CREATE_TIME,
    ctime: CREATE_TIME,
    crtime: CREATE_TIME,
    kind: FileType::Directory,
    perm: 0o755,
    nlink: 2,
    uid: 501,
    gid: 20,
    rdev: 0,
    flags: 0,
};

const HELLO_FILE_ATTR: FileAttr = FileAttr {
    ino: 1,
    size: 0,
    blocks: 0,
    atime: CREATE_TIME,
    mtime: CREATE_TIME,
    ctime: CREATE_TIME,
    crtime: CREATE_TIME,
    kind: FileType::RegularFile,
    perm: 0o755,
    nlink: 2,
    uid: 501,
    gid: 20,
    rdev: 0,
    flags: 0,
};

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use std::io::Cursor;

    fn default_request_info() -> RequestInfo {
        RequestInfo {
            unique: 0,
            uid: 0,
            gid: 0,
            pid: 0,
        }
    }

    fn one_file_in_flat_zip(name: &str, content: &str) -> Vec<u8> {
        multiple_files_in_flat_zip(&vec![(name, content)])
    }

    fn multiple_files_in_flat_zip(files: &[(&str, &str)]) -> Vec<u8> {
        let mut z = zip::ZipWriter::new(Cursor::new(vec![]));

        for &(name, content) in files {
            z.start_file(name, zip::write::FileOptions::default()).unwrap();
            z.write(content.as_bytes()).unwrap();
        }

        z.finish().unwrap().into_inner()
    }

    fn get_attr<X:Read + Seek>(path: &str, arch: &ArchiveFs<X>) -> fuse_mt::FileAttr {
        let path = Path::new(path);
        arch.getattr(default_request_info(), path, None).unwrap().1
    }

    fn assert_attr_dir<X:Read + Seek>(path: &str, arch: &ArchiveFs<X>) {
        let attr = get_attr(path, arch);
        match attr.kind {
            fuse_mt::FileType::Directory => return,
            _ => panic!("Expected Directory got {:?}", attr.kind),
        }
    }

    fn assert_attr_file<X:Read + Seek>(path: &str, arch: &ArchiveFs<X>) {
        let attr = get_attr(path, arch);
        match attr.kind {
            fuse_mt::FileType::RegularFile => return,
            _ => panic!("Expected RegularFile got {:?}", attr.kind),
        }
    }

    fn assert_attr_size<X : Read + Seek>(expected: u64, path: &str, arch: &ArchiveFs<X>) {
        let attr = get_attr(path, arch);
        assert_eq!(expected, attr.size);
    }

    fn assert_file_content<X : Read + Seek>(expected: &str, path: &str, arch: &ArchiveFs<X>) {
        let path = Path::new(path);
        let v = arch.read(default_request_info(), path, 0, 0, expected.len() as u32).unwrap();
        assert_eq!(expected.as_bytes(), v.as_slice());
    }

    fn assert_is_file(name: &str, entry: &fuse_mt::DirectoryEntry) {
        assert_eq!(*name, entry.name);
        assert_eq!(fuse_mt::FileType::RegularFile, entry.kind);
    }

    fn assert_is_directory(name: &str, entry: &fuse_mt::DirectoryEntry) {
        assert_eq!(*name, entry.name);
        assert_eq!(fuse_mt::FileType::Directory, entry.kind);
    }

    #[test]
    fn readdir_on_one_file_in_flat_zip() {
        let buf = one_file_in_flat_zip("test.txt", "content");
        let archive = ArchiveFs::from_zip(Cursor::new(buf)).unwrap();

        let result = archive.readdir(default_request_info(), Path::new("/"), 0).unwrap();
        assert_eq!(1, result.len());
        assert_is_file("test.txt", &result[0]);

        let result = archive.readdir(default_request_info(), Path::new("/"), 0).unwrap();
        assert_eq!(1, result.len());
        assert_is_file("test.txt", &result[0]);
    }

    #[test]
    fn readdir_multiple_files_in_flat_zip() {
        let files = vec![("test.txt", "content"), ("foo", "asdsa"), ("bar", ""), ("baz", "xxxxx")];
        let buf = multiple_files_in_flat_zip(&files);
        let archive = ArchiveFs::from_zip(Cursor::new(buf)).unwrap();

        let listing = archive.readdir(default_request_info(), Path::new("/"), 0).unwrap();

        assert_eq!(files.len(), listing.len());
        for i in 0..files.len() {
            assert_is_file(files[i].0, &listing[i]);
        }
    }

    fn multiple_files_in_multiple_directories_zip() -> Vec<u8> {
        let mut z = zip::ZipWriter::new(Cursor::new(vec![]));

        z.start_file("top.txt", zip::write::FileOptions::default()).unwrap();
        z.write("top_content".as_bytes()).unwrap();

        z.start_file("a\\inner.txt", zip::write::FileOptions::default()).unwrap();
        z.write("inner_content".as_bytes()).unwrap();

        z.start_file("b\\inner.txt", zip::write::FileOptions::default()).unwrap();
        z.write("inner_content2".as_bytes()).unwrap();

        z.start_file("a\\other.txt", zip::write::FileOptions::default()).unwrap();
        z.write("other_content".as_bytes()).unwrap();

        z.start_file("b\\c\\deep.txt", zip::write::FileOptions::default()).unwrap();
        z.write("deep_content".as_bytes()).unwrap();

        z.finish().unwrap().into_inner()
    }

    #[test]
    fn readdir_multiple_files_in_multiple_directories_zip() {
        let buf = multiple_files_in_multiple_directories_zip();
        let archive = ArchiveFs::from_zip(Cursor::new(buf)).unwrap();

        let listing = archive.readdir(default_request_info(), Path::new("/"), 0).unwrap();

        assert_eq!(3, listing.len());

        assert_is_file("top.txt", &listing[0]);
        assert_is_directory("a", &listing[1]);
        assert_is_directory("b", &listing[2]);
    }

    #[test]
    fn readdir_subdir_multiple_files_in_multiple_directories_zip() {
        let buf = multiple_files_in_multiple_directories_zip();
        let archive = ArchiveFs::from_zip(Cursor::new(buf)).unwrap();

        let listing = archive.readdir(default_request_info(), Path::new("/b"), 0).unwrap();

        assert_eq!(2, listing.len());
        assert_is_file("inner.txt", &listing[0]);
        assert_is_directory("c", &listing[1]);
    }

    #[test]
    fn test_convert_name() {
        assert_eq!(Some("b"),
                   convert_name("b", Path::new("/")).unwrap().name.to_str());
        assert_eq!(Some("b"),
                   convert_name("b\\c", Path::new("/")).unwrap().name.to_str());
        assert!(convert_name("b", Path::new("/b")).is_none());
        assert_eq!(Some("c"),
                   convert_name("b\\c", Path::new("/b")).unwrap().name.to_str());
        assert_eq!(Some("c"),
                   convert_name("b\\c\\d", Path::new("/b")).unwrap().name.to_str());
        assert_eq!(fuse_mt::FileType::Directory,
                   convert_name("b\\c\\d", Path::new("/b")).unwrap().kind);
        assert_eq!(Some("d"),
                   convert_name("b\\c\\d", Path::new("/b/c")).unwrap().name.to_str());

        assert_eq!(None, convert_name("b", Path::new("/c")).map(|e| e.name));
        assert_eq!(None, convert_name("b/c", Path::new("/b/a")).map(|e| e.name));
    }

    #[test]
    fn getattr_subdir_multiple_files_in_multiple_directories_zip() {
        let buf = multiple_files_in_multiple_directories_zip();
        let archive = ArchiveFs::from_zip(Cursor::new(buf)).unwrap();

        assert_attr_dir("/a", &archive);
        assert_attr_dir("/b", &archive);
        assert_attr_dir("/b/c", &archive);

        assert_attr_file("/top.txt", &archive);
        assert_attr_size(11, "/top.txt", &archive);
        assert_file_content("top_content", "/top.txt", &archive);

        assert_attr_file("/a/inner.txt", &archive);
        assert_attr_size(13, "/a/inner.txt", &archive);
        assert_file_content("inner_content", "/a/inner.txt", &archive);

        assert_attr_file("/b/inner.txt", &archive);
        assert_attr_size(14, "/b/inner.txt", &archive);
        assert_file_content("inner_content2", "/b/inner.txt", &archive);

        assert_attr_file("/a/other.txt", &archive);
        assert_attr_size(13, "/a/other.txt", &archive);
        assert_file_content("other_content", "/a/other.txt", &archive);

        assert_attr_file("/b/c/deep.txt", &archive);
        assert_attr_size(12, "/b/c/deep.txt", &archive);
        assert_file_content("deep_content", "/b/c/deep.txt", &archive);
    }
}
