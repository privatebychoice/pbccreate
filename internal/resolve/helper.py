#!/usr/bin/env python3
# pbccreate DaVinci Resolve scripting helper (SPEC section 8.2).
#
# This is the ONLY integration route with Resolve: the Go process invokes this
# script with python3, passes one JSON command on stdin, and reads one JSON
# result on stdout. It imports Blackmagic's DaVinciResolveScript module (Resolve
# Studio only) and drives a RUNNING Resolve instance. It holds no secrets and
# opens no network connections.
#
# The SQLite plan is the source of truth; Resolve is a sink. On any failure the
# helper reports {"ok": false, ...} and exits 0 so the Go side can read the JSON,
# leaving local state untouched.
import json
import sys


def respond(payload):
    json.dump(payload, sys.stdout)
    sys.stdout.flush()
    sys.exit(0)


def fail(message, code="error"):
    respond({"ok": False, "error": message, "code": code})


def get_or_add_folder(media_pool, parent, name):
    for sub in parent.GetSubFolderList() or []:
        if sub.GetName() == name:
            return sub
    return media_pool.AddSubFolder(parent, name)


def index_clips_by_path(folder, out):
    for clip in folder.GetClipList() or []:
        path = clip.GetClipProperty("File Path")
        if path:
            out.setdefault(path, clip)
    for sub in folder.GetSubFolderList() or []:
        index_clips_by_path(sub, out)


def main():
    try:
        req = json.load(sys.stdin)
    except Exception as exc:  # noqa: BLE001 - report any parse failure as data
        fail("invalid request json: %s" % exc, "bad_request")

    action = req.get("action", "")

    try:
        import DaVinciResolveScript as bmd
    except Exception as exc:  # noqa: BLE001
        fail("DaVinciResolveScript import failed: %s" % exc, "no_module")

    resolve = bmd.scriptapp("Resolve")
    if resolve is None:
        fail("Resolve is not running or not reachable", "no_resolve")

    if action == "ping":
        try:
            desc = "%s %s" % (resolve.GetProductName(), resolve.GetVersionString())
        except Exception:  # noqa: BLE001
            desc = "connected"
        respond({"ok": True, "message": desc})

    project_manager = resolve.GetProjectManager()
    if project_manager is None:
        fail("no project manager available", "no_project_manager")

    if action == "create_project":
        spec = req.get("project") or {}
        name = spec.get("name") or "Untitled"
        project = project_manager.LoadProject(name) or project_manager.CreateProject(name)
        if project is None:
            fail("could not create or open project %r" % name, "project_failed")
        respond({"ok": True, "message": "project ready: %s" % project.GetName()})

    if action == "import_media":
        project = project_manager.GetCurrentProject()
        if project is None:
            fail("no current project (create or open one first)", "no_project")
        media_pool = project.GetMediaPool()
        root = media_pool.GetRootFolder()
        total = 0
        for spec in req.get("bins") or []:
            bin_name = spec.get("name") or "Footage"
            folder = get_or_add_folder(media_pool, root, bin_name)
            media_pool.SetCurrentFolder(folder)
            clips = spec.get("clips") or []
            if clips:
                added = media_pool.ImportMedia(clips) or []
                total += len(added)
        respond({"ok": True, "message": "imported %d clip(s)" % total})

    if action == "build_timeline":
        project = project_manager.GetCurrentProject()
        if project is None:
            fail("no current project (create or open one first)", "no_project")
        media_pool = project.GetMediaPool()
        spec = req.get("timeline") or {}
        name = spec.get("name") or "Timeline"
        timeline = media_pool.CreateEmptyTimeline(name)
        if timeline is None:
            fail("could not create timeline %r" % name, "timeline_failed")

        index = {}
        index_clips_by_path(media_pool.GetRootFolder(), index)
        items = [index[p] for p in (spec.get("clips") or []) if p in index]
        if items:
            media_pool.AppendToTimeline(items)
        respond({
            "ok": True,
            "message": "timeline %r built with %d clip(s)" % (name, len(items)),
        })

    fail("unknown action %r" % action, "unknown_action")


if __name__ == "__main__":
    main()
