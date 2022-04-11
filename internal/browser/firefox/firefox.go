package firefox

// type firefox struct {
// 	name           string
// 	storage        string
// 	profilePath    string
// 	masterKey      []byte
// 	items          []item.Item
// 	itemPaths      map[item.Item]string
// 	multiItemPaths map[string]map[item.Item]string
// }
//
// // New
// func New(info *browserInfo, items []item.Item) ([]*firefox, error) {
// 	f := &firefox{
// 		browserInfo: info,
// 		items:       items,
// 	}
// 	multiItemPaths, err := getFirefoxItemAbsPath(f.browserInfo.profilePath, f.items)
// 	if err != nil {
// 		if strings.Contains(err.Error(), "profile path is not exist") {
// 			fmt.Println(err)
// 			return nil, nil
// 		}
// 		panic(err)
// 	}
// 	var firefoxList []*firefox
// 	for name, value := range multiItemPaths {
// 		firefoxList = append(firefoxList, &firefox{
// 			browserInfo: &browserInfo{
// 				name:      name,
// 				masterKey: nil,
// 			},
// 			items:     items,
// 			itemPaths: value,
// 		})
// 	}
// 	return firefoxList, nil
// }
//
// func getFirefoxItemAbsPath(profilePath string, items []item.Item) (map[string]map[item.Item]string, error) {
// 	var multiItemPaths = make(map[string]map[item.Item]string)
// 	absProfilePath := path.Join(homeDir, filepath.Clean(profilePath))
// 	// TODO: Handle read file error
// 	if !isFileExist(absProfilePath) {
// 		return nil, fmt.Errorf("%s profile path is not exist", absProfilePath)
// 	}
// 	err := filepath.Walk(absProfilePath, firefoxWalkFunc(items, multiItemPaths))
// 	return multiItemPaths, err
// }
//
// func (f *firefox) CopyItemFileToLocal() error {
// 	for item, sourcePath := range f.itemPaths {
// 		var dstFilename = item.TempName()
// 		locals, _ := filepath.Glob("*")
// 		for _, v := range locals {
// 			if v == dstFilename {
// 				err := os.Remove(dstFilename)
// 				// TODO: Should Continue all iteration error
// 				if err != nil {
// 					return err
// 				}
// 			}
// 		}
//
// 		// TODO: Handle read file name error
// 		sourceFile, err := ioutil.ReadFile(sourcePath)
// 		if err != nil {
// 			fmt.Println(err.Error())
// 		}
// 		err = ioutil.WriteFile(dstFilename, sourceFile, 0777)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
//
// func firefoxWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) filepath.WalkFunc {
// 	return func(path string, info fs.FileInfo, err error) error {
// 		for _, v := range items {
// 			if info.Name() == v.FileName() {
// 				parentDir := getParentDir(path)
// 				if _, exist := multiItemPaths[parentDir]; exist {
// 					multiItemPaths[parentDir][v] = path
// 				} else {
// 					multiItemPaths[parentDir] = map[item.Item]string{v: path}
// 				}
// 			}
// 		}
// 		return err
// 	}
// }
//
// func getParentDir(absPath string) string {
// 	return filepath.Base(filepath.Dir(absPath))
// }
//
// func (f *firefox) GetMasterKey() ([]byte, error) {
// 	return f.masterKey, nil
// }
//
// func (f *firefox) GetName() string {
// 	return f.name
// }
//
// func (f *firefox) GetBrowsingData() []browingdata.Source {
// 	var browsingData []browingdata.Source
// 	for item := range f.itemPaths {
// 		d := item.NewBrowsingData()
// 		if d != nil {
// 			browsingData = append(browsingData, d)
// 		}
// 	}
// 	return browsingData
// }
