package main

// Client-side glue: keyboard handling for the block editor (enter/backspace,
// "/" menu, tab indent, arrow navigation), draft preservation across
// collaborative re-renders, and drag & drop. Everything else (state,
// rendering, persistence) lives on the server.
const glueJS = `
<script>
(function(){
	var draft = {bid:null, cell:null, value:null};
	var committed = false;
	var slashOpen = false;

	window.lvDraftSave = function(el){
		draft = {bid: el.dataset.bid, cell: el.dataset.cell||null, value: el.value};
		if(el.tagName==='TEXTAREA'){ el.style.height='auto'; el.style.height=el.scrollHeight+'px'; }
		if(el.dataset.kind==='block'){
			if(el.value.charAt(0)==='/'){
				slashOpen = true;
				send_event('ed','Slash', el.dataset.bid+'|'+el.value.slice(1));
			} else if(slashOpen){
				slashOpen = false;
				send_event('ed','SlashClose','');
			}
		}
	};

	function commit(el, enter){
		if(committed) return;
		committed = true;
		if(el.dataset.kind==='cell'){
			send_event('ed','CommitCell', el.dataset.bid+'|'+el.dataset.cell+'|'+el.value);
		}else{
			send_event('ed','Commit', el.dataset.bid+'|'+(enter?'1':'0')+'|'+el.value);
		}
		draft = {bid:null, cell:null, value:null};
		slashOpen = false;
	}

	document.addEventListener('keydown', function(e){
		var el = document.activeElement;
		if(!el) return;
		if((e.ctrlKey||e.metaKey) && e.key.toLowerCase()==='k'){
			e.preventDefault();
			var q = document.getElementById('nt_search');
			if(q) q.focus();
			return;
		}
		if(el.id==='title_input' && e.key==='Enter'){ el.blur(); return; }
		if(el.id!=='blk_edit') return;
		var isCode = el.dataset.btype==='code';

		if(e.key==='Enter' && slashOpen && el.value.charAt(0)==='/'){
			e.preventDefault(); committed = true; slashOpen = false;
			send_event('ed','SlashPick', el.dataset.bid+'|'+el.value.slice(1));
			draft = {bid:null, cell:null, value:null};
			return;
		}
		if(e.key==='Tab' && el.dataset.kind==='block'){
			e.preventDefault();
			send_event('ed','Indent', el.dataset.bid+'|'+(e.shiftKey?'-1':'1'));
			return;
		}
		if(e.key==='ArrowUp' && el.dataset.kind==='block' && el.selectionStart===0 && el.selectionEnd===0 && !slashOpen){
			e.preventDefault(); commit(el,false);
			send_event('ed','EditPrev', el.dataset.bid);
			return;
		}
		if(e.key==='ArrowDown' && el.dataset.kind==='block' && el.selectionStart===el.value.length && !slashOpen){
			e.preventDefault(); commit(el,false);
			send_event('ed','EditNext', el.dataset.bid);
			return;
		}
		if(e.key==='Enter' && !e.shiftKey && (!isCode || e.ctrlKey || e.metaKey)){
			e.preventDefault(); commit(el, true);
		} else if(e.key==='Escape'){
			e.preventDefault(); commit(el, false);
		} else if(e.key==='Backspace' && el.value==='' && el.dataset.kind==='block'){
			e.preventDefault(); committed = true;
			send_event('ed','DelMerge', el.dataset.bid);
		}
	}, true);

	document.addEventListener('focusout', function(e){
		if(e.target && e.target.id==='blk_edit' && document.contains(e.target)){
			commit(e.target, false);
		}
	}, true);

	// After every server re-render: refocus the editing input, restore the
	// local draft (so a collaborator's commit never wipes what you typed),
	// and put the cursor at the end.
	function watch(){
		var box = document.getElementById('editor_box');
		var head = document.getElementById('page_head');
		if(!box || !head){ setTimeout(watch, 300); return; }
		new MutationObserver(function(){
			var el = document.getElementById('blk_edit');
			if(!el) return;
			committed = false;
			if(draft.value!==null && draft.bid===el.dataset.bid && draft.cell===(el.dataset.cell||null) && el.value!==draft.value){
				el.value = draft.value;
			}
			if(document.activeElement!==el){
				el.focus();
				try{ el.setSelectionRange(el.value.length, el.value.length); }catch(_){}
			}
			if(el.tagName==='TEXTAREA'){ el.style.height='auto'; el.style.height=el.scrollHeight+'px'; }
		}).observe(box, {childList:true, subtree:true});
	}
	watch();

	// ---- drag & drop ----
	var dragSrc = null;
	window.lvDragStart = function(e, id){ dragSrc = id; e.dataTransfer.effectAllowed='move'; };
	window.lvDragOver = function(e, blk){
		if(!dragSrc) return;
		e.preventDefault();
		var r = blk.getBoundingClientRect();
		var before = (e.clientY - r.top) < r.height/2;
		blk.classList.toggle('nt-drop-before', before);
		blk.classList.toggle('nt-drop-after', !before);
	};
	window.lvDragLeave = function(blk){ blk.classList.remove('nt-drop-before','nt-drop-after'); };
	window.lvDrop = function(e, blk){
		e.preventDefault();
		var before = blk.classList.contains('nt-drop-before');
		blk.classList.remove('nt-drop-before','nt-drop-after');
		if(dragSrc && dragSrc!==blk.dataset.bid){
			send_event('ed','MoveBlk', dragSrc+'|'+blk.dataset.bid+'|'+(before?'before':'after'));
		}
		dragSrc = null;
	};
})();
</script>`

const notionCss = `
html, body, #content { height: 100%; margin: 0; }
.nt-app { display: flex; height: 100vh; font-family: ui-sans-serif, -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; color: #37352f; }

/* sidebar */
.nt-side { width: 260px; background: #f7f6f3; border-right: 1px solid #ececea; display: flex; flex-direction: column; }
.nt-logo { padding: 14px 16px 8px; font-weight: 700; font-size: .95rem; display: flex; gap: 8px; align-items: center; }
.sb-search { padding: 4px 8px 8px; }
.sb-search input { width: 100%; box-sizing: border-box; border: 1px solid #e3e2e0; background: #fff; border-radius: 8px; padding: 6px 10px; font-size: .85rem; outline: none; }
.sb-search input:focus { border-color: #b9b8b5; }
.sb-section { font-size: .7rem; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; color: #9b9a97; padding: 10px 12px 4px; }
.sb-empty { color: #b9b8b5; font-size: .85rem; padding: 4px 12px; }
.nt-tree { flex: 1; overflow-y: auto; padding: 0 4px 8px; }
.sb-row { display: flex; align-items: center; gap: 5px; padding: 4px 8px; border-radius: 6px; cursor: pointer; font-size: .88rem; color: #5f5e5b; }
.sb-row:hover { background: #ececea; }
.sb-active { background: #ececea; color: #37352f; font-weight: 600; }
.sb-caret { width: 14px; text-align: center; color: #9b9a97; font-size: .75rem; border-radius: 4px; }
.sb-caret:hover { background: #ddd; }
.sb-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sb-act { visibility: hidden; color: #9b9a97; border-radius: 4px; padding: 0 3px; font-size: .8rem; }
.sb-row:hover .sb-act { visibility: visible; }
.sb-act:hover { background: #ddd; color: #37352f; }
.sb-hit { align-items: flex-start; }
.sb-hitbody { flex: 1; min-width: 0; }
.sb-snippet { font-size: .75rem; color: #9b9a97; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sb-snippet b { color: #37352f; background: #ffe9a8; border-radius: 2px; }
.nt-newpage { margin: 8px; padding: 7px 10px; border-radius: 6px; cursor: pointer; color: #5f5e5b; font-size: .88rem; border: none; background: none; text-align: left; }
.nt-newpage:hover { background: #ececea; }

/* main */
.nt-main { flex: 1; display: flex; flex-direction: column; min-width: 0; background: #fff; }
.nt-topbar { display: flex; justify-content: space-between; align-items: center; padding: 10px 18px; font-size: .85rem; }
.nt-crumb { cursor: pointer; padding: 2px 6px; border-radius: 4px; color: #5f5e5b; }
.nt-crumb:hover { background: #ececea; }
.nt-crumb-sep { color: #c8c7c4; margin: 0 2px; }
.nt-topright { display: flex; align-items: center; gap: 10px; }
.nt-ago { color: #b9b8b5; font-size: .78rem; }
.nt-star { cursor: pointer; font-size: 1rem; border-radius: 4px; padding: 2px 4px; }
.nt-star:hover { background: #ececea; }
.nt-presence { display: flex; gap: 4px; }
.nt-avatar { width: 24px; height: 24px; border-radius: 50%; color: #fff; font-size: .7rem; font-weight: 700; display: inline-flex; align-items: center; justify-content: center; border: 2px solid #fff; box-shadow: 0 0 0 1px #ddd; }

.nt-doc { flex: 1; overflow-y: auto; }
.nt-cover { height: 190px; position: relative; }
.nt-coveracts { position: absolute; right: 18px; bottom: 10px; display: flex; gap: 6px; opacity: 0; transition: opacity .15s; }
.nt-cover:hover .nt-coveracts { opacity: 1; }
.nt-coverbar { max-width: 720px; margin: 0 auto; padding: 18px 24px 0; }
.nt-coverbtn { background: rgba(255,255,255,.92); border: 1px solid #e3e2e0; border-radius: 6px; padding: 3px 10px; font-size: .78rem; cursor: pointer; color: #5f5e5b; }
.nt-coverbtn:hover { background: #fff; color: #37352f; }
.nt-page { max-width: 720px; margin: 0 auto; padding: 14px 24px 30vh; }
.nt-cover + .nt-page, .nt-doc .nt-cover ~ .nt-page { padding-top: 0; }
.nt-icon { font-size: 3.2rem; cursor: pointer; width: fit-content; border-radius: 8px; padding: 2px 6px; margin-top: -34px; background: transparent; }
.nt-icon:hover { background: #f1f0ee; }
.nt-iconmenu { background: #fff; border: 1px solid #e3e2e0; border-radius: 10px; box-shadow: 0 8px 24px rgba(0,0,0,.12); padding: 8px; display: inline-block; position: relative; z-index: 40; }
.nt-iconmenu span { font-size: 1.4rem; padding: 4px; cursor: pointer; border-radius: 6px; display: inline-block; }
.nt-iconmenu span:hover { background: #f1f0ee; }
.nt-title { font-size: 2.4rem; font-weight: 800; border: none; outline: none; width: 100%; padding: 6px 0 14px; color: #37352f; background: transparent; }
.nt-title::placeholder { color: #d3d2cf; }

/* blocks */
.nt-blk { display: flex; align-items: flex-start; position: relative; border-radius: 4px; margin: 1px 0; }
.nt-gut { width: 62px; display: flex; justify-content: flex-end; gap: 1px; padding-top: 5px; visibility: hidden; flex-shrink: 0; margin-left: -18px; }
.nt-blk:hover .nt-gut { visibility: visible; }
.nt-plus, .nt-dots, .nt-handle { color: #b9b8b5; cursor: pointer; border-radius: 4px; padding: 0 3px; font-size: .85rem; user-select: none; }
.nt-plus:hover, .nt-dots:hover, .nt-handle:hover { background: #ececea; color: #5f5e5b; }
.nt-handle { cursor: grab; letter-spacing: -2px; }
.nt-body { flex: 1; min-width: 0; padding: 2px 4px; cursor: text; border-radius: 4px; }
.nt-drop-before { box-shadow: 0 -3px 0 #4f9cf7; }
.nt-drop-after { box-shadow: 0 3px 0 #4f9cf7; }
.nt-editing { position: absolute; right: 0; top: -10px; color: #fff; font-size: .65rem; padding: 1px 7px; border-radius: 9px; z-index: 2; }

.nt-p, .nt-li, .nt-todo { font-size: .95rem; line-height: 1.6; margin: 0; }
.nt-empty { color: #d3d2cf; }
h1, h2, h3 { margin: .4em 0 .2em; font-weight: 700; }
h1 { font-size: 1.8rem; } h2 { font-size: 1.4rem; } h3 { font-size: 1.15rem; }
.nt-li { display: flex; gap: 8px; }
.nt-marker { color: #37352f; min-width: 14px; }
.nt-togglec { cursor: pointer; border-radius: 4px; }
.nt-togglec:hover { background: #ececea; }
.nt-todo { display: flex; gap: 8px; align-items: baseline; }
.nt-todo input { accent-color: #2383e2; cursor: pointer; }
.nt-done { text-decoration: line-through; color: #9b9a97; }
blockquote { border-left: 3px solid #37352f; margin: 2px 0; padding: 2px 0 2px 12px; font-size: .95rem; }
.nt-code { background: #f7f6f3; border: 1px solid #ececea; border-radius: 8px; padding: 14px; font-size: .85rem; overflow-x: auto; margin: 4px 0; }
.nt-callout { background: #f1f0ee; border-radius: 8px; padding: 12px 14px; font-size: .92rem; margin: 2px 0; }
.nt-hr { border: none; border-top: 1px solid #e3e2e0; margin: 10px 0; }
.nt-img { max-width: 100%; border-radius: 8px; margin: 4px 0; }
.nt-imgempty { background: #f1f0ee; border-radius: 8px; padding: 14px; color: #9b9a97; font-size: .9rem; }
code { background: #f1f0ee; color: #eb5757; border-radius: 4px; padding: 1px 5px; font-size: .85em; }
.nt-code code { background: none; color: #37352f; padding: 0; }

.nt-input { width: 100%; border: none; outline: none; resize: none; font-family: inherit; font-size: .95rem; line-height: 1.6; background: #f6f9ff; border-radius: 4px; padding: 2px 4px; color: #37352f; overflow: hidden; box-sizing: border-box; }
.nt-input-h1 { font-size: 1.8rem; font-weight: 700; }
.nt-input-h2 { font-size: 1.4rem; font-weight: 700; }
.nt-input-h3 { font-size: 1.15rem; font-weight: 700; }
.nt-input-code { font-family: ui-monospace, monospace; font-size: .85rem; background: #f7f6f3; }

.nt-table { border-collapse: collapse; margin: 6px 0; width: 100%; font-size: .9rem; }
.nt-table th, .nt-table td { border: 1px solid #e3e2e0; padding: 6px 10px; min-width: 70px; cursor: pointer; }
.nt-table th { background: #f7f6f3; font-weight: 600; }
.nt-table td:hover, .nt-table th:hover { background: #f6f9ff; }
.nt-cellinput { border: none; outline: none; width: 100%; font: inherit; background: #f6f9ff; }
.nt-tablebar { display: flex; gap: 12px; font-size: .78rem; color: #9b9a97; padding: 2px 0 6px; }
.nt-tablebar span { cursor: pointer; border-radius: 4px; padding: 1px 6px; }
.nt-tablebar span:hover { background: #ececea; color: #37352f; }

.nt-menu { position: absolute; left: 62px; top: 28px; background: #fff; border: 1px solid #e3e2e0; border-radius: 10px; box-shadow: 0 10px 30px rgba(0,0,0,.15); z-index: 50; min-width: 210px; padding: 5px; max-height: 320px; overflow-y: auto; }
.nt-menu-item { padding: 6px 10px; border-radius: 6px; cursor: pointer; font-size: .88rem; display: flex; align-items: center; gap: 10px; }
.nt-menu-item:hover { background: #f1f0ee; }
.nt-mi-ico { width: 24px; height: 24px; border: 1px solid #e3e2e0; border-radius: 5px; display: inline-flex; align-items: center; justify-content: center; font-size: .75rem; background: #fff; flex-shrink: 0; }
.nt-mi-first { background: #f1f0ee; }
.nt-menu-del { color: #eb5757; border-top: 1px solid #ececea; margin-top: 4px; }
.nt-slash::before { content: "Escribí para filtrar — Enter elige el primero"; display: block; font-size: .68rem; color: #b9b8b5; padding: 4px 10px 6px; }

.nt-addblk { color: #b9b8b5; cursor: pointer; padding: 8px 4px 8px 48px; font-size: .9rem; border-radius: 6px; }
.nt-addblk:hover { background: #f7f6f3; color: #5f5e5b; }
.nt-subpages { margin-top: 28px; border-top: 1px solid #ececea; padding-top: 12px; }
.nt-sub-h { font-size: .75rem; color: #9b9a97; text-transform: uppercase; letter-spacing: .05em; margin-bottom: 6px; }
.nt-sublink { padding: 5px 6px; border-radius: 6px; cursor: pointer; font-size: .92rem; }
.nt-sublink:hover { background: #f1f0ee; }
a { color: inherit; }
`
