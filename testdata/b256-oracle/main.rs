use alloy_primitives::B256;
use std::str::FromStr;

// Prints, for each input, whether the pinned Rust build decodes it as a B256
// (FromStr and JSON) and the decoded hash.
fn main() {
    let d = "11".repeat(32);
    let up = d.to_uppercase();
    let abc = "ab".repeat(32);
    let abc_mixed: String = abc.chars().enumerate().map(|(i, c)| if i % 3 == 0 { c.to_ascii_uppercase() } else { c }).collect();
    let inputs: Vec<(&str, String)> = vec![
        ("lowercase 0x", format!("0x{d}")),
        ("bare", d.clone()),
        ("uppercase 0X", format!("0X{d}")),
        ("0x, upper digits", format!("0x{}", "AB".repeat(32))),
        ("bare, upper digits", "AB".repeat(32)),
        ("0x, mixed digits", format!("0x{abc_mixed}")),
        ("bare, mixed digits", abc_mixed.clone()),
        ("0X, upper digits", format!("0X{up}")),
        ("0x, 63 digits", format!("0x{}", &d[1..])),
        ("0x, 65 digits", format!("0x{d}1")),
        ("0x, 66 digits", format!("0x{d}11")),
        ("0x, non-hex", format!("0x{}g", &d[1..])),
        ("double 0x", format!("0x0x{}", &d[4..])),
        ("empty", String::new()),
        ("0x only", "0x".to_string()),
        ("leading space", format!(" 0x{d}")),
        ("x only prefix", format!("x{d}")),
    ];
    for (name, s) in inputs {
        let from_str = B256::from_str(&s).ok();
        let json = serde_json::from_str::<B256>(&serde_json::to_string(&s).unwrap()).ok();
        let show = |v: Option<B256>| v.map(|h| format!("{h:x}")).unwrap_or_else(|| "refused".into());
        println!("{name}\t{s}\t{}\t{}", show(from_str), show(json));
    }
}
