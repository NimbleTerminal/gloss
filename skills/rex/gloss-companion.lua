-- gloss-companion.lua: keeps a gloss split in step with the block it sits
-- beside. Written by `gloss skill install rex`; started by the gloss_companion
-- action in gloss.lua, or by hand:
--
--   rex do ~/.config/rex/gloss-companion.lua [file=PATH] [label=NAME]
--
-- It opens the split with `gloss --socket NAME TARGET`, then, as the shell's
-- folder changes (terminal.pwd_changed) or the editor's title names another
-- file (terminal.title_changed), tells the viewer with `gloss --remote NAME
-- TARGET`, which works whatever the viewer is doing. When that fails because
-- the viewer is gone, the split is opened again. The script ends when either
-- block closes.
--
-- Arguments: source_block (default: the block this runs in), label (default
-- gloss-preview), socket (default: derived from the session and label, as
-- gloss.lua derives it), gloss (the binary to run; default: gloss on the PATH),
-- file (what to show first; default: the folder's README or the folder itself),
-- page (the page of that first file).

local TERMINAL = "com.superlogical.terminal"
local SHELL_FLAVOR = "com.superlogical.terminal.shell"
local LABEL = rex.args.label or "gloss-preview"
local DEBOUNCE = 0.12

local session_id = rex.session_id
local source_block_id = rex.args.source_block or rex.block_id
local preview_block_id = nil
local last_target = nil
local pending = nil

if not source_block_id then
  error("gloss-companion needs a block to sit beside: run it from a terminal, or give source_block=", 0)
end

-- What gloss shows well when an editor's title names it.
local PREVIEWABLE = {
  md = true, markdown = true, pdf = true, html = true, htm = true, ipynb = true,
  svg = true, png = true, jpg = true, jpeg = true, webp = true, gif = true, bmp = true, tiff = true, heic = true, jxl = true,
  csv = true, tsv = true, json = true, jsonl = true, ndjson = true, txt = true, xlsx = true, docx = true, grist = true,
  stl = true, ["3mf"] = true, obj = true, ply = true, geojson = true, gpx = true, pmtiles = true, mmd = true, mermaid = true,
  mp4 = true, mov = true, webm = true, mkv = true,
}

-- What to show on entering a folder, first found wins; else the folder.
local LANDING = { "README.md", "readme.md", "README.markdown", "README", "index.md", "ARCHITECTURE.md" }

-- socket_name must agree with gloss.lua's; see there.
local function socket_name(sid, label)
  local id = (sid or ""):gsub("[^%w]", "")
  id = id:sub(-12)
  local name = (label or "gloss-preview"):gsub("[^%w%._-]", "-"):sub(1, 40)
  return "rex-" .. id .. "-" .. name
end

local SOCKET = rex.args.socket or socket_name(session_id, LABEL)
local GLOSS = rex.args.gloss or "gloss"

local function file_exists(path)
  local f = io.open(path, "r")
  if f then
    f:close()
    return true
  end
  return false
end

-- from_url turns the file:// URL that OSC 7 reports into a path; a plain path
-- comes back as it is.
local function from_url(pwd)
  if not pwd or pwd == "" then
    return nil
  end
  pwd = pwd:gsub("^file://[^/]*", "")
  pwd = pwd:gsub("%%(%x%x)", function(hex)
    return string.char(tonumber(hex, 16))
  end)
  return pwd
end

-- source_dir is the folder the source block last reported.
local function source_dir(block_id)
  local res = rex.block.call(TERMINAL, "pwd", { block_id = block_id or source_block_id })
  return from_url(res and res.pwd) or os.getenv("HOME") or "."
end

-- landing is what to show for a folder.
local function landing(dir)
  if not dir or dir == "" then
    return nil
  end
  for _, name in ipairs(LANDING) do
    local candidate = dir .. "/" .. name
    if file_exists(candidate) then
      return candidate
    end
  end
  return dir
end

-- file_in_title is a file the title names that gloss can show, or nil. Editors
-- with `title` on put the buffer's name there, relative to their folder.
local function file_in_title(title, dir)
  if not title or title == "" then
    return nil
  end
  for token in title:gmatch("[%w%._%-/~]+%.%w+") do
    local ext = token:match("%.(%w+)$")
    if ext and PREVIEWABLE[ext:lower()] then
      local path = token
      if path:sub(1, 1) == "~" then
        path = (os.getenv("HOME") or "") .. path:sub(2)
      elseif path:sub(1, 1) ~= "/" and dir then
        path = dir .. "/" .. path
      end
      if file_exists(path) then
        return path
      end
    end
  end
  return nil
end

local function trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

-- open_split puts the viewer beside the source block, listening on SOCKET.
-- The first viewer starts on the page asked for, if any; later targets do not.
local first_page = rex.args.page
local function open_split(target)
  target = target or landing(source_dir()) or "."
  local command = { GLOSS, "--socket", SOCKET, target }
  if first_page then
    table.insert(command, "--page")
    table.insert(command, tostring(first_page))
    first_page = nil
  end
  local split, err = rex.session.new_split{
    block_id = source_block_id,
    direction = "horizontal",
    side = "after",
    focus = false,
    layout = rex.layout.block{
      flavor = SHELL_FLAVOR,
      options = {
        command = command,
        cwd = source_dir(),
        -- A gloss too old for --socket, or no gloss, fails at once: leave
        -- the error on screen rather than close the split without a word.
        exit = { quick_exit_threshold_ms = 2000 },
      },
    },
  }
  if not split then
    error("opening the gloss split: " .. tostring(err), 0)
  end
  preview_block_id = split.block_ids[1]
  last_target = target
  rex.session.set_block_label{ block_id = preview_block_id, label = LABEL }
  return preview_block_id
end

-- show tells the viewer to open target; a viewer that is gone is replaced.
local function show(target)
  if not target or target == "" or target == last_target then
    return
  end
  if not preview_block_id then
    open_split(target)
    return
  end
  local res = rex.exec({ GLOSS, "--remote", SOCKET, target }, { timeout = 40 })
  if res and res.ok then
    last_target = target
    return
  end
  local reason = trim(res and res.stderr)
  if reason:find("no gloss viewer is listening", 1, true) or reason:find("quitting", 1, true) then
    -- The viewer left; its block closes with it. Open another.
    preview_block_id = nil
    open_split(target)
    return
  end
  rex.log("warn", "gloss did not take " .. target .. ": " .. reason)
end

-- schedule shows target once the events settle.
local function schedule(target)
  if pending then
    pending:cancel()
  end
  pending = rex.after(DEBOUNCE, function()
    pending = nil
    show(target)
  end)
end

open_split(rex.args.file)

rex.on("terminal.pwd_changed", function(ctx, ev)
  if ctx.block_id ~= source_block_id then
    return
  end
  schedule(landing(from_url(ev.pwd)))
end)

rex.on("terminal.title_changed", function(ctx, ev)
  if ctx.block_id ~= source_block_id then
    return
  end
  local path = file_in_title(ev.title, source_dir(ctx.block_id))
  if path then
    schedule(path)
  end
end)

rex.on("block_closed", function(ctx, ev)
  local closed = ev.block_id or ctx.block_id
  if closed == source_block_id or closed == preview_block_id then
    rex.stop(0)
  end
end)
