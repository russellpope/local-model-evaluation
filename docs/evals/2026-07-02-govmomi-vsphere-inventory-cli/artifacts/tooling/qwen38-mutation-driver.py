#!/usr/bin/env python3
"""Full mutation battery, isolated tree. apply -> grep-verify -> test -> revert -> grep-verify."""
import json
import os
import shutil
import subprocess

BASE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(BASE, "qwen3.8-27b-bf16", "vsphere-inventory")
BACKUP = os.path.join(BASE, "pristine")
DIRS = ("cmd", "config", "inventory")

M = [
    ("M01", "ClassifyTransport short-circuits: always FC", "inventory/transport.go",
     [('func ClassifyTransport(datastoreType string, hba HBADescriptor) string {\n\tif strings.EqualFold',
       'func ClassifyTransport(datastoreType string, hba HBADescriptor) string {\n\treturn TransportFC // MUTANT M01\n\tif strings.EqualFold', 1)]),
    ("M02", "ClassifyTransport short-circuits: always unknown", "inventory/transport.go",
     [('func ClassifyTransport(datastoreType string, hba HBADescriptor) string {\n\tif strings.EqualFold',
       'func ClassifyTransport(datastoreType string, hba HBADescriptor) string {\n\treturn TransportUnknown // MUTANT M02\n\tif strings.EqualFold', 1)]),
    ("M03", "Invert FC <-> iSCSI in the HBA-kind switch", "inventory/transport.go",
     [('\tcase "fibrechannel", "fibrechannelethernet", "fc", "fcoe":\n\t\treturn TransportFC\n\tcase "internetscsi", "iscsi":\n\t\treturn TransportISCSI',
       '\tcase "fibrechannel", "fibrechannelethernet", "fc", "fcoe":\n\t\treturn TransportISCSI // MUTANT M03\n\tcase "internetscsi", "iscsi":\n\t\treturn TransportFC // MUTANT M03', 1)]),
    ("M04", "Delete the NFS-wins-over-HBA rule", "inventory/transport.go",
     [('\tif strings.EqualFold(strings.TrimSpace(datastoreType), "nfs") {\n\t\treturn TransportNFS\n\t}\n\n',
       '\t// MUTANT M04: NFS rule deleted\n\n', 1)]),
    ("M05", "GiB divisor 1<<30 -> 1<<29", "inventory/format.go",
     [('\tgib = int64(1) << 30', '\tgib = int64(1) << 29 // MUTANT M05', 1)]),
    ("M06", "FormatBytes rounding %.1f -> %.0f", "inventory/format.go",
     [('return fmt.Sprintf("%.1fGiB", gibf)', 'return fmt.Sprintf("%.0fGiB", gibf) // MUTANT M06', 1)]),
    ("M07", "FormatBytes returns a constant", "inventory/format.go",
     [('func FormatBytes(b int64) string {\n\tif b < 0 {',
       'func FormatBytes(b int64) string {\n\treturn "1.0GiB" // MUTANT M07\n\tif b < 0 {', 1)]),
    ("M08", "UsedBytes returns the zero value", "inventory/format.go",
     [('func UsedBytes(total, available int64) int64 {\n\tused := total - available',
       'func UsedBytes(total, available int64) int64 {\n\treturn 0 // MUTANT M08\n\tused := total - available', 1)]),
    ("M09", "VM RAM hardcoded to a plausible 4 GiB", "inventory/vm.go",
     [('info.RAMBytes = int64(vm.Config.Hardware.MemoryMB) * mib', 'info.RAMBytes = 4 * gib // MUTANT M09', 1)]),
    ("M10", "VM vCPU hardcoded to 2", "inventory/vm.go",
     [('info.VCPU = int(vm.Config.Hardware.NumCPU)', 'info.VCPU = 2 // MUTANT M10', 1)]),
    ("M11", "committedBytes body deleted, returns 0", "inventory/vm.go",
     [('func committedBytes(vm *mo.VirtualMachine) int64 {\n\tif vm.Summary.Storage != nil',
       'func committedBytes(vm *mo.VirtualMachine) int64 {\n\treturn 0 // MUTANT M11\n\tif vm.Summary.Storage != nil', 1)]),
    ("M12", "committedBytes hardcoded to a plausible 42 GiB", "inventory/vm.go",
     [('func committedBytes(vm *mo.VirtualMachine) int64 {\n\tif vm.Summary.Storage != nil',
       'func committedBytes(vm *mo.VirtualMachine) int64 {\n\treturn 42 * gib // MUTANT M12\n\tif vm.Summary.Storage != nil', 1)]),
    ("M13", "collectVMs swallows the API error, returns nil slice", "inventory/vm.go",
     [('return nil, fmt.Errorf("collect virtual machine properties: %w", err)', 'return nil, nil // MUTANT M13', 1)]),
    ("M14", "ListVMs sorts descending", "inventory/vm.go",
     [('sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })\n\treturn infos, nil',
       'sort.Slice(infos, func(i, j int) bool { return infos[i].Name > infos[j].Name }) // MUTANT M14\n\treturn infos, nil', 1)]),
    ("M15", "Datastore capacity hardcoded to 1 TiB", "inventory/datastore.go",
     [('CapacityBytes:  ds.Summary.Capacity,', 'CapacityBytes:  1 << 40, // MUTANT M15', 1)]),
    ("M16", "Capacity/available/used ALL fabricated but self-consistent", "inventory/datastore.go",
     [('\t\tused := UsedBytes(ds.Summary.Capacity, ds.Summary.FreeSpace)\n\t\tinfos = append(infos, DatastoreInfo{\n\t\t\tName:           ds.Name,\n\t\t\tType:           datastoreTransport(hosts, ds),\n\t\t\tCapacityBytes:  ds.Summary.Capacity,\n\t\t\tAvailableBytes: ds.Summary.FreeSpace,\n\t\t\tUsedBytes:      used,\n\t\t})',
       '\t\tused := UsedBytes(ds.Summary.Capacity, ds.Summary.FreeSpace)\n\t\t_ = used // MUTANT M16\n\t\tinfos = append(infos, DatastoreInfo{\n\t\t\tName:           ds.Name,\n\t\t\tType:           datastoreTransport(hosts, ds),\n\t\t\tCapacityBytes:  1 << 40, // MUTANT M16\n\t\t\tAvailableBytes: 400 << 30, // MUTANT M16\n\t\t\tUsedBytes:      624 << 30, // MUTANT M16\n\t\t})', 1)]),
    ("M17", "Datastore used = capacity (UsedBytes bypassed)", "inventory/datastore.go",
     [('used := UsedBytes(ds.Summary.Capacity, ds.Summary.FreeSpace)', 'used := ds.Summary.Capacity // MUTANT M17', 1)]),
    ("M18", "Datastore TYPE hardcoded to FC, real traversal bypassed", "inventory/datastore.go",
     [('hosts, err := hostStorages(ctx, c)', '_, err = hostStorages(ctx, c) // MUTANT M18', 1),
      ('Type:           datastoreTransport(hosts, ds),', 'Type:           TransportFC, // MUTANT M18', 1)]),
    ("M19", "datastoreTransport body deleted -> always unknown", "inventory/datastore.go",
     [('func datastoreTransport(hosts []*hostStorage, ds *mo.Datastore) string {\n\tif strings.EqualFold',
       'func datastoreTransport(hosts []*hostStorage, ds *mo.Datastore) string {\n\treturn TransportUnknown // MUTANT M19\n\tif strings.EqualFold', 1)]),
    ("M20", "vmfsUUID body deleted -> always empty", "inventory/datastore.go",
     [('func vmfsUUID(ds *mo.Datastore) string {\n\tif ds.Info == nil {',
       'func vmfsUUID(ds *mo.Datastore) string {\n\treturn "" // MUTANT M20\n\tif ds.Info == nil {', 1)]),
    ("M21", "hostStorages swallows the API error, returns nil", "inventory/datastore.go",
     [('return nil, fmt.Errorf("collect host storage properties: %w", err)', 'return nil, nil // MUTANT M21', 1)]),
    ("M22", "hbaDescriptor hardcodes every HBA class to fibrechannel", "inventory/datastore.go",
     [('\t\tKind:            "other",', '\t\tKind:            "fibrechannel", // MUTANT M22', 1)]),
    ("M23", "Portgroup<->switch NAME-based fallback matching removed", "inventory/switch.go",
     [('\t\t\t\tpg, ok := pgByKey[ref]\n\t\t\t\tif !ok {\n\t\t\t\t\tpg, ok = pgByName[ref]\n\t\t\t\t}',
       '\t\t\t\tpg, ok := pgByKey[ref]\n\t\t\t\t_ = pgByName // MUTANT M23: name fallback removed', 1)]),
    ("M24", "Portgroup<->switch KEY index never populated", "inventory/switch.go",
     [('\t\t\tpgByKey[pg.Key] = pg\n\t\t\tpgByName[pg.Spec.Name] = pg',
       '\t\t\t_ = pg.Key // MUTANT M24: key index not populated\n\t\t\tpgByName[pg.Spec.Name] = pg', 1)]),
    ("M25", "Standard portgroup VLAN hardcoded to 100", "inventory/switch.go",
     [('VLAN:       strconv.FormatInt(int64(pg.Spec.VlanId), 10),', 'VLAN:       "100", // MUTANT M25', 1)]),
    ("M26", "dvpgVLAN body deleted -> always \"0\"", "inventory/switch.go",
     [('func dvpgVLAN(setting types.BaseDVPortSetting) string {\n\tvmware, ok := setting.(*types.VMwareDVSPortSetting)',
       'func dvpgVLAN(setting types.BaseDVPortSetting) string {\n\treturn "0" // MUTANT M26\n\tvmware, ok := setting.(*types.VMwareDVSPortSetting)', 1)]),
    ("M27", "DVS total port count hardcoded to 6144", "inventory/switch.go",
     [('\t\ttotal := int(cfg.MaxPorts)', '\t\ttotal := 6144 // MUTANT M27', 1)]),
    ("M28", "DVS used port count hardcoded to 8", "inventory/switch.go",
     [('\t\tused := int(cfg.NumPorts)', '\t\tused := 8 // MUTANT M28', 1)]),
    ("M29", "Standard vSwitch total ports hardcoded to 1024 (both emit sites)", "inventory/switch.go",
     [('Ports:      int(sw.NumPorts),', 'Ports:      1024, // MUTANT M29', 2)]),
    ("M30", "Standard vSwitch used ports hardcoded to 0", "inventory/switch.go",
     [('used := UsedBytes(int64(sw.NumPorts), int64(sw.NumPortsAvailable))', 'used := int64(0) // MUTANT M30', 1)]),
    ("M31", "DVS LACP always reported enabled", "inventory/switch.go",
     [('\t\tlacp := "disabled"', '\t\tlacp := "enabled" // MUTANT M31', 1)]),
    ("M32", "DVS uplink names dropped", "inventory/switch.go",
     [('\t\t\tif name, ok := nameByRef[ref.Value]; ok {\n\t\t\t\tuplinks = append(uplinks, name)\n\t\t\t}',
       '\t\t\t_ = ref // MUTANT M32: uplink names dropped', 1)]),
    ("M33", "vmInPortgroup always true (filter disabled)", "inventory/portgroup.go",
     [('func vmInPortgroup(vm *mo.VirtualMachine, match map[string]bool) bool {\n\tfor _, ref := range vm.Network {',
       'func vmInPortgroup(vm *mo.VirtualMachine, match map[string]bool) bool {\n\treturn true // MUTANT M33\n\tfor _, ref := range vm.Network {', 1)]),
    ("M34", "vmInPortgroup always false", "inventory/portgroup.go",
     [('func vmInPortgroup(vm *mo.VirtualMachine, match map[string]bool) bool {\n\tfor _, ref := range vm.Network {',
       'func vmInPortgroup(vm *mo.VirtualMachine, match map[string]bool) bool {\n\treturn false // MUTANT M34\n\tfor _, ref := range vm.Network {', 1)]),
    ("M35", "Portgroup name match ignores the requested name", "inventory/portgroup.go",
     [('\t\t\tif objs[i].Name == name {', '\t\t\tif objs[i].Name != "" { // MUTANT M35', 1)]),
    ("M36", "viewRefs swallows the Find error, returns nil slice", "inventory/collect.go",
     [('\trefs, err := v.Find(ctx, nil, nil)\n\tif err != nil {\n\t\treturn nil, err\n\t}',
       '\trefs, err := v.Find(ctx, nil, nil)\n\tif err != nil {\n\t\treturn nil, nil // MUTANT M36\n\t}', 1)]),
    ("M37", "Precedence swap: flags demoted to the DEFAULT layer (flag < env)", "config/config.go",
     [('\t\tif err := v.BindPFlag(key, f); err != nil {\n\t\t\treturn fmt.Errorf("bind flag --%s: %w", key, err)\n\t\t}',
       '\t\tif f.Changed { // MUTANT M37: flag becomes the DEFAULT layer\n\t\t\tv.SetDefault(key, f.Value.String())\n\t\t}', 1)]),
    ("M38", "Precedence swap: env layer disabled (env < file)", "config/config.go",
     [('\tv.AutomaticEnv()', '\t// v.AutomaticEnv() // MUTANT M38', 1)]),
    ("M39", "Load ignores the resolved timeout, returns the built-in default", "config/config.go",
     [('\tcfg.Timeout = d', '\tcfg.Timeout = DefaultTimeout // MUTANT M39', 1)]),
    ("M40", "Load skips the required-URL check", "config/config.go",
     [('\tif cfg.URL == "" {\n\t\treturn Config{}, fmt.Errorf("no vCenter URL configured; set --url, the %s_URL environment variable, or url in the config file", EnvPrefix)\n\t}',
       '\t// MUTANT M40: required-URL check deleted', 1)]),
    ("M41", "parseVCenterURL builds an unusable ftp:// URL", "cmd/root.go",
     [('\t\ts = "https://" + s', '\t\ts = "ftp://" + s // MUTANT M41', 1)]),
    ("M42", "runWithClient never attaches credentials", "cmd/root.go",
     [('\tif cfg.Username != "" {', '\tif false && cfg.Username != "" { // MUTANT M42', 1)]),
    ("M43", "standardSwitches returns nothing (whole path deleted)", "inventory/switch.go",
     [('func standardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {\n\trefs, err := viewRefs',
       'func standardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {\n\treturn nil, nil // MUTANT M43\n\trefs, err := viewRefs', 1)]),
    ("M44", "distributedSwitches returns nothing (whole DVS path deleted)", "inventory/switch.go",
     [('func distributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {\n\tv, err := rootView',
       'func distributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {\n\treturn nil, nil // MUTANT M44\n\tv, err := rootView', 1)]),
    ("M45", "newHostStorage body deleted: no volumes, no LUN topology, no HBAs", "inventory/datastore.go",
     [('\tcfg := host.Config\n\tif cfg == nil {\n\t\treturn hs\n\t}',
       '\treturn hs // MUTANT M45\n\tcfg := host.Config\n\tif cfg == nil {\n\t\treturn hs\n\t}', 1)]),
    ("M46", "joinOrUnknown always returns \"unknown\"", "inventory/switch.go",
     [('func joinOrUnknown(items []string) string {\n\tif len(items) == 0 {',
       'func joinOrUnknown(items []string) string {\n\treturn "unknown" // MUTANT M46\n\tif len(items) == 0 {', 1)]),
    ("M47", "hbaDescriptor returns the zero descriptor", "inventory/datastore.go",
     [('func hbaDescriptor(hba types.BaseHostHostBusAdapter, base *types.HostHostBusAdapter) HBADescriptor {\n\tdesc := HBADescriptor{',
       'func hbaDescriptor(hba types.BaseHostHostBusAdapter, base *types.HostHostBusAdapter) HBADescriptor {\n\treturn HBADescriptor{} // MUTANT M47\n\tdesc := HBADescriptor{', 1)]),
    ("M48", "ListDatastores never fetches host storage", "inventory/datastore.go",
     [('\thosts, err := hostStorages(ctx, c)\n\tif err != nil {\n\t\treturn nil, err\n\t}',
       '\tvar hosts []*hostStorage // MUTANT M48: host storage never fetched', 1)]),
    ("M49", "VMFS volume<->datastore matching disabled", "inventory/datastore.go",
     [('\tuuid := vmfsUUID(ds)\n\tfor _, hs := range hosts {\n\t\tfor _, vol := range hs.volumes {\n\t\t\tif vol.name != ds.Name && (uuid == "" || vol.uuid != uuid) {\n\t\t\t\tcontinue\n\t\t\t}',
       '\tuuid := vmfsUUID(ds)\n\t_ = uuid // MUTANT M49\n\tfor _, hs := range hosts {\n\t\tfor _, vol := range hs.volumes {\n\t\t\tif true { // MUTANT M49: volume matching disabled\n\t\t\t\tcontinue\n\t\t\t}', 1)]),
    ("M50", "LUN -> adapter topology walk deleted", "inventory/datastore.go",
     [('\t\t\t\t\t\tif _, ok := hs.lunToAdapter[lun.ScsiLun]; ok {\n\t\t\t\t\t\t\ths.lunToAdapter[lun.ScsiLun] = iface.Adapter\n\t\t\t\t\t\t}',
       '\t\t\t\t\t\t_ = lun // MUTANT M50: topology walk deleted', 1)]),
    ("M51", "VMFS volume matching inverted", "inventory/datastore.go",
     [('\t\t\tif vol.name != ds.Name && (uuid == "" || vol.uuid != uuid) {',
       '\t\t\tif vol.name == ds.Name || (uuid != "" && vol.uuid == uuid) { // MUTANT M51', 1)]),
    ("M52", "networkViewTypes reduced to Network only (DVPG subtype dropped)", "inventory/portgroup.go",
     [('var networkViewTypes = []string{\n\t"Network",\n\t"DistributedVirtualPortgroup",\n\t"OpaqueNetwork",\n}',
       'var networkViewTypes = []string{ // MUTANT M52\n\t"Network",\n}', 1)]),
    ("M53", "VM property list drops config.hardware.memoryMB", "inventory/vm.go",
     [('\t\t"config.hardware.memoryMB",\n', '\t\t// MUTANT M53: memoryMB property dropped\n', 1)]),
    ("M54", "VM property list drops the network property", "inventory/vm.go",
     [('\t\t"network",\n', '\t\t// MUTANT M54: network property dropped\n', 1)]),
    ("M55", "DVS portgroup retrieval drops config (VLAN source)", "inventory/switch.go",
     [('\t\t\t\t[]string{"name", "config"}, &pgs); err != nil {', '\t\t\t\t[]string{"name"}, &pgs); err != nil { // MUTANT M55', 1)]),
]


def restore():
    """File-by-file overwrite; no rmtree, so a concurrent reader cannot race us."""
    for d in DIRS:
        for fn in os.listdir(os.path.join(BACKUP, d)):
            shutil.copy2(os.path.join(BACKUP, d, fn), os.path.join(ROOT, d, fn))


def run(cmd):
    p = subprocess.run(cmd, cwd=ROOT, shell=True, capture_output=True, text=True)
    return p.returncode, (p.stdout + p.stderr).strip()


def context_of(path, marker, n=2):
    lines = open(path).read().split("\n")
    out = []
    for i, ln in enumerate(lines):
        if marker in ln:
            for j in range(max(0, i - n), min(len(lines), i + n + 1)):
                out.append("%s:%d: %s" % (os.path.basename(path), j + 1, lines[j]))
            out.append("")
    return "\n".join(out) if out else "!! MARKER NOT FOUND !!"


def main():
    if not os.path.isdir(BACKUP):
        os.makedirs(BACKUP)
        for d in DIRS:
            shutil.copytree(os.path.join(ROOT, d), os.path.join(BACKUP, d))

    results = []
    for mid, desc, relpath, edits in M:
        restore()
        path = os.path.join(ROOT, relpath)
        ok = True
        for old, new, count in edits:
            src = open(path).read()
            if src.count(old) != count:
                results.append({"id": mid, "desc": desc, "file": relpath, "status": "PATCH-FAILED",
                                "detail": "anchor occurs %d times, expected %d" % (src.count(old), count)})
                ok = False
                break
            open(path, "w").write(src.replace(old, new))
        if not ok:
            print("%s %-14s %s" % (mid, "PATCH-FAILED", desc), flush=True)
            continue

        rc, gout = run("grep -n 'MUTANT %s' %s" % (mid, relpath))
        grep_txt = ("$ grep -n 'MUTANT %s' %s\n%s\n\n" % (mid, relpath, gout or "(NO MATCH)")
                    + context_of(path, "MUTANT " + mid))
        landed = bool(gout) and "!! MARKER NOT FOUND !!" not in grep_txt

        brc, bout = run("go build ./...")
        if brc != 0:
            status, tout = "COMPILE-ERROR", bout
        else:
            trc, tout = run("go test ./...")
            status = "CAUGHT" if trc != 0 else "SURVIVED"

        restore()
        rrc, rout = run("grep -rn 'MUTANT' . --include=*.go")
        results.append({"id": mid, "desc": desc, "file": relpath, "status": status,
                        "landed": landed, "grep": grep_txt, "test_output": tout[-3000:],
                        "revert_clean": rout == ""})
        print("%s %-14s landed=%-5s revert_clean=%-5s %s"
              % (mid, status, landed, rout == "", desc), flush=True)

    json.dump(results, open(os.path.join(BASE, "results_final.json"), "w"), indent=1)


main()
