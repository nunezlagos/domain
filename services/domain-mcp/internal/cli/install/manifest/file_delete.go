package manifest

import "os"

type FileDeleteReverser struct{}

func (r *FileDeleteReverser) CanRevert(entry Entry) bool {
	return entry.RevertStrategy == "delete_file" || entry.RevertStrategy == "remove_symlink"
}

func (r *FileDeleteReverser) Revert(entry Entry) error {
	err := os.Remove(entry.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
