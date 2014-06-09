var io = new Image();
var pageAction, sale, price, sku, order_code, currency_id, user_defined1, user_defined2, user_defined3, user_defined4, ic_cat, ic_bu, ic_bc, ic_ch, ic_nso, altid, ic_type, urlA, prefix;
function pixel()
{
	var icstring =".ic-live.com/goat.php?cID=1055&cdid=6078&campID=8";
	var refVar = (document.referrer);
	var locURL = location.href;
	var locHttp = locURL.split(":")[0];

	if (!pageAction) { pageAction = 0; };
	if (!sale) { sale=""; }
	if (!price) { price=""; }
	if (!sku) { sku=""; }
	if (!order_code) { order_code=""; }
	if (!user_defined1) { user_defined1=""; }
	if (!user_defined2) { user_defined2=""; }
	if (!user_defined3) { user_defined3=""; }
	if (!user_defined4) { user_defined4=""; }
	if (!currency_id) { currency_id=""; }
	if (!ic_bu) { ic_bu=""; }
	if (!ic_bc) { ic_bc=""; }
	if (!ic_ch) { ic_ch=""; }
	if (!ic_nso) { ic_nso=""; }
	if (!altid) { altid=""; }
	if (!ic_cat) { ic_cat=""; }
	if (!ic_type) { ic_type=""; }
	if (locHttp.toLowerCase( ) == "https")  { prefix="https://www"+icstring+"";}
	if (locHttp.toLowerCase( ) == "http")  { prefix="http://1055"+icstring+"";}

	if (pageAction > 0) {
		urlA = prefix+"&convID="+pageAction+"&sl="+sale+"&convP="+price+"&curID="+currency_id+"&ordID="+escape(order_code)+"&ud1="+escape(user_defined1)+"&ud2="+escape(user_defined2)+"&ud3="+escape(user_defined3)+"&ud4="+escape(user_defined4)+"&ic_cat="+escape(ic_cat)+"&ic_type="+escape(ic_type)+"&ic_bu="+escape(ic_bu)+"&ic_bc="+escape(ic_bc)+"&ic_ch="+escape(ic_ch)+"&ic_nso="+escape(ic_nso)+"&altid="+escape(altid)+"&sku="+escape(sku)+"&refVar="+escape(refVar);
	} else {
		urlA = prefix+"&ic_cat="+escape(ic_cat)+"&ic_type="+escape(ic_type)+"&ic_bu="+escape(ic_bu)+"&ic_bc="+escape(ic_bc)+"&ic_ch="+escape(ic_ch)+"&ic_nso="+escape(ic_nso)+"&altid="+escape(altid)+"&refVar="+escape(refVar);
	}
	io.src = urlA;
}
pixel();